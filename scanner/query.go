package scanner

import (
	"container/heap"
	"encoding/csv"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Row is one line in the tree view. Key is "d<id>" for folders and "f<id>:<idx>" for files.
type Row struct {
	Key     string `json:"key"`
	Name    string `json:"name"`
	IsDir   bool   `json:"isDir"`
	Size    int64  `json:"size"`
	Alloc   int64  `json:"alloc"`
	Files   int64  `json:"files"`
	Dirs    int64  `json:"dirs"`
	MTime   int64  `json:"mtime"`
	HasKids bool   `json:"hasKids"`
	Done    bool   `json:"done"`
	Err     string `json:"err,omitempty"`
	Linked  bool   `json:"linked,omitempty"`
	Ext     string `json:"ext,omitempty"`
}

func DirRow(d *Dir) Row {
	dirs, files := d.Dirs(), d.Files()
	return Row{
		Key: "d" + strconv.Itoa(int(d.ID)), Name: d.Name, IsDir: true,
		Size: d.Size(), Alloc: d.Alloc(), Files: d.FileCount(), Dirs: d.DirCount(), MTime: d.MTime(),
		HasKids: len(dirs)+len(files) > 0, Done: d.Done(), Err: d.Err(), Linked: d.Linked,
	}
}

func fileRow(d *Dir, i int, f File) Row {
	return Row{
		Key: "f" + strconv.Itoa(int(d.ID)) + ":" + strconv.Itoa(i), Name: f.Name,
		Size: f.Size, Alloc: f.Alloc, Files: 1, MTime: f.MTime, Done: true, Ext: Ext(f.Name),
	}
}

// Ext returns the lower-case extension without dot ("" if none).
// Trailing numeric parts are treated as version numbers: "lib.so.1.2" -> "so".
func Ext(name string) string {
	parts := strings.Split(strings.ToLower(name), ".")
	if len(parts) < 2 || parts[0] == "" {
		return ""
	}
	for len(parts) > 2 && isDigits(parts[len(parts)-1]) {
		parts = parts[:len(parts)-1]
	}
	e := parts[len(parts)-1]
	if e == "" || len(e) > 16 || isDigits(e) {
		return ""
	}
	return e
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// Resolve parses a row key into a dir (and file index, -1 for folders).
func (t *Tree) Resolve(key string) (*Dir, int, bool) {
	if len(key) < 2 {
		return nil, -1, false
	}
	switch key[0] {
	case 'd':
		id, err := strconv.Atoi(key[1:])
		if err != nil {
			return nil, -1, false
		}
		d := t.Node(uint32(id))
		return d, -1, d != nil
	case 'f':
		a, b, ok := strings.Cut(key[1:], ":")
		if !ok {
			return nil, -1, false
		}
		id, e1 := strconv.Atoi(a)
		idx, e2 := strconv.Atoi(b)
		if e1 != nil || e2 != nil {
			return nil, -1, false
		}
		d := t.Node(uint32(id))
		if d == nil || idx < 0 || idx >= len(d.Files()) {
			return nil, -1, false
		}
		return d, idx, true
	}
	return nil, -1, false
}

// KeyPath returns the full path for a row key.
func (t *Tree) KeyPath(key string) (string, bool) {
	d, idx, ok := t.Resolve(key)
	if !ok {
		return "", false
	}
	p := t.PathOf(d)
	if idx >= 0 {
		p = joinPath(p, d.Files()[idx].Name)
	}
	return p, true
}

// Children lists the entries of a folder, sorted.
func (t *Tree) Children(d *Dir, sortBy string, desc, withFiles bool) []Row {
	dirs, files := d.Dirs(), d.Files()
	rows := make([]Row, 0, len(dirs)+len(files))
	for _, c := range dirs {
		rows = append(rows, DirRow(c))
	}
	if withFiles {
		for i, f := range files {
			rows = append(rows, fileRow(d, i, f))
		}
	}
	SortRows(rows, sortBy, desc)
	return rows
}

func SortRows(rows []Row, sortBy string, desc bool) {
	less := func(a, b Row) bool {
		switch sortBy {
		case "name":
			return strings.ToLower(a.Name) < strings.ToLower(b.Name)
		case "alloc":
			return a.Alloc < b.Alloc
		case "files":
			return a.Files < b.Files
		case "dirs":
			return a.Dirs < b.Dirs
		case "mtime":
			return a.MTime < b.MTime
		default:
			return a.Size < b.Size
		}
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if desc {
			return less(rows[j], rows[i])
		}
		return less(rows[i], rows[j])
	})
}

// MapNode is a treemap node. Compact JSON keys keep transfers small.
type MapNode struct {
	Key  string     `json:"k,omitempty"`
	Name string     `json:"n"`
	Size int64      `json:"s"`
	Dir  bool       `json:"d,omitempty"`
	Ext  string     `json:"e,omitempty"`
	Rest int        `json:"r,omitempty"` // >0: aggregated "N small items"
	Kids []*MapNode `json:"c,omitempty"`
}

// Treemap returns a nested size map below d, limited to maxNodes entries.
// Items smaller than minFrac of d's size are merged into one "rest" node per folder.
func (t *Tree) Treemap(d *Dir, maxNodes int, minFrac float64) *MapNode {
	total := d.Size()
	minSize := int64(float64(total) * minFrac)
	root := &MapNode{Key: "d" + strconv.Itoa(int(d.ID)), Name: d.Name, Size: total, Dir: true}
	queue := &mapQueue{{root, d}}
	count := 1
	for queue.Len() > 0 && count < maxNodes {
		it := heap.Pop(queue).(mapItem)
		var kids []*MapNode
		var restSize int64
		var restN int
		for _, c := range it.d.Dirs() {
			s := c.Size()
			if s <= 0 {
				continue
			}
			if s < minSize {
				restSize += s
				restN++
				continue
			}
			n := &MapNode{Key: "d" + strconv.Itoa(int(c.ID)), Name: c.Name, Size: s, Dir: true}
			kids = append(kids, n)
			heap.Push(queue, mapItem{n, c})
		}
		for i, f := range it.d.Files() {
			if f.Size <= 0 {
				continue
			}
			if f.Size < minSize {
				restSize += f.Size
				restN++
				continue
			}
			kids = append(kids, &MapNode{Key: "f" + strconv.Itoa(int(it.d.ID)) + ":" + strconv.Itoa(i), Name: f.Name, Size: f.Size, Ext: Ext(f.Name)})
		}
		if restN > 0 {
			kids = append(kids, &MapNode{Name: "", Size: restSize, Rest: restN})
		}
		sort.Slice(kids, func(i, j int) bool { return kids[i].Size > kids[j].Size })
		it.n.Kids = kids
		count += len(kids)
	}
	return root
}

// mapQueue expands the largest folders first so the node budget goes where it is visible.
type mapItem struct {
	n *MapNode
	d *Dir
}
type mapQueue []mapItem

func (q mapQueue) Len() int           { return len(q) }
func (q mapQueue) Less(i, j int) bool { return q[i].n.Size > q[j].n.Size }
func (q mapQueue) Swap(i, j int)      { q[i], q[j] = q[j], q[i] }
func (q *mapQueue) Push(x any)        { *q = append(*q, x.(mapItem)) }
func (q *mapQueue) Pop() any {
	old := *q
	x := old[len(old)-1]
	*q = old[:len(old)-1]
	return x
}

// walkFiles calls fn for every file below d.
func walkFiles(d *Dir, fn func(d *Dir, i int, f *File)) {
	files := d.Files()
	for i := range files {
		fn(d, i, &files[i])
	}
	for _, c := range d.Dirs() {
		walkFiles(c, fn)
	}
}

// ExtStat aggregates files by extension.
type ExtStat struct {
	Ext   string `json:"ext"`
	Count int64  `json:"count"`
	Size  int64  `json:"size"`
}

func (t *Tree) Extensions(d *Dir, limit int) []ExtStat {
	m := map[string]*ExtStat{}
	walkFiles(d, func(_ *Dir, _ int, f *File) {
		e := Ext(f.Name)
		s := m[e]
		if s == nil {
			s = &ExtStat{Ext: e}
			m[e] = s
		}
		s.Count++
		s.Size += f.Size
	})
	out := make([]ExtStat, 0, len(m))
	for _, s := range m {
		out = append(out, *s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Size > out[j].Size })
	if limit > 0 && len(out) > limit {
		var rest ExtStat
		rest.Ext = "*"
		for _, s := range out[limit:] {
			rest.Count += s.Count
			rest.Size += s.Size
		}
		out = append(out[:limit], rest)
	}
	return out
}

// FileInfo is a file with its folder path, used for the "largest files" list.
type FileInfo struct {
	Row
	Folder string `json:"folder"`
}

type fileHeap []FileInfo

func (h fileHeap) Len() int           { return len(h) }
func (h fileHeap) Less(i, j int) bool { return h[i].Size < h[j].Size }
func (h fileHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }
func (h *fileHeap) Push(x any)        { *h = append(*h, x.(FileInfo)) }
func (h *fileHeap) Pop() any {
	old := *h
	x := old[len(old)-1]
	*h = old[:len(old)-1]
	return x
}

// TopFiles returns the n largest files below d. ext filters by extension when not empty.
func (t *Tree) TopFiles(d *Dir, n int, ext string) []FileInfo {
	h := &fileHeap{}
	type ref struct {
		d *Dir
		i int
	}
	walkFiles(d, func(dd *Dir, i int, f *File) {
		if ext != "" && Ext(f.Name) != ext && !(ext == "-" && Ext(f.Name) == "") {
			return
		}
		if h.Len() < n {
			heap.Push(h, FileInfo{Row: fileRow(dd, i, *f)})
		} else if f.Size > (*h)[0].Size {
			(*h)[0] = FileInfo{Row: fileRow(dd, i, *f)}
			heap.Fix(h, 0)
		}
	})
	out := make([]FileInfo, h.Len())
	for i := len(out) - 1; i >= 0; i-- {
		out[i] = heap.Pop(h).(FileInfo)
	}
	for i := range out {
		if dd, _, ok := t.Resolve(out[i].Key); ok {
			out[i].Folder = t.PathOf(dd)
		}
	}
	return out
}

// AgeBucket groups files by last modification.
type AgeBucket struct {
	ID    string `json:"id"`
	Count int64  `json:"count"`
	Size  int64  `json:"size"`
}

func (t *Tree) Ages(d *Dir, now time.Time) []AgeBucket {
	limits := []struct {
		id  string
		age time.Duration
	}{
		{"1m", 30 * 24 * time.Hour},
		{"6m", 182 * 24 * time.Hour},
		{"1y", 365 * 24 * time.Hour},
		{"2y", 2 * 365 * 24 * time.Hour},
		{"5y", 5 * 365 * 24 * time.Hour},
		{"old", 1 << 62},
	}
	out := make([]AgeBucket, len(limits))
	for i, l := range limits {
		out[i].ID = l.id
	}
	nowS := now.Unix()
	walkFiles(d, func(_ *Dir, _ int, f *File) {
		age := time.Duration(nowS-f.MTime) * time.Second
		for i, l := range limits {
			if age < l.age || i == len(limits)-1 {
				out[i].Count++
				out[i].Size += f.Size
				break
			}
		}
	})
	return out
}

// ExportCSV writes all folders below d (up to maxDepth, 0 = unlimited) as CSV.
func (t *Tree) ExportCSV(w io.Writer, d *Dir, maxDepth int, withFiles bool, sep rune) error {
	cw := csv.NewWriter(w)
	cw.Comma = sep
	if err := cw.Write([]string{"Path", "Type", "Size", "Allocated", "Files", "Folders", "Modified", "Percent of parent"}); err != nil {
		return err
	}
	ts := func(u int64) string {
		if u <= 0 {
			return ""
		}
		return time.Unix(u, 0).Format("2006-01-02 15:04:05")
	}
	pct := func(s, p int64) string {
		if p <= 0 {
			return "0"
		}
		return strconv.FormatFloat(float64(s)*100/float64(p), 'f', 2, 64)
	}
	var rec func(dd *Dir, path string, depth int, parentSize int64) error
	rec = func(dd *Dir, path string, depth int, parentSize int64) error {
		if err := cw.Write([]string{path, "folder", fmt.Sprint(dd.Size()), fmt.Sprint(dd.Alloc()),
			fmt.Sprint(dd.FileCount()), fmt.Sprint(dd.DirCount()), ts(dd.MTime()), pct(dd.Size(), parentSize)}); err != nil {
			return err
		}
		if maxDepth > 0 && depth >= maxDepth {
			return nil
		}
		rows := t.Children(dd, "size", true, withFiles)
		for _, r := range rows {
			cd, idx, ok := t.Resolve(r.Key)
			if !ok {
				continue
			}
			if idx >= 0 {
				if err := cw.Write([]string{joinPath(path, r.Name), "file", fmt.Sprint(r.Size), fmt.Sprint(r.Alloc),
					"1", "0", ts(r.MTime), pct(r.Size, dd.Size())}); err != nil {
					return err
				}
				continue
			}
			if err := rec(cd, joinPath(path, cd.Name), depth+1, dd.Size()); err != nil {
				return err
			}
		}
		return nil
	}
	if err := rec(d, t.PathOf(d), 0, 0); err != nil {
		return err
	}
	cw.Flush()
	return cw.Error()
}
