package main

import (
	"image"
	"strings"
	"time"

	"gioui.org/font"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/text"

	"sizescope/scanner"
)

func (u *UI) sizeOf(r scanner.Row) int64 {
	if u.settings.SizeMode == "alloc" {
		return r.Alloc
	}
	return r.Size
}

func (u *UI) rebuildFlat() {
	t := u.tree
	u.flatDirty = false
	u.flatAt = time.Now()
	if t == nil {
		u.flat = nil
		return
	}
	out := u.flat[:0]
	out = append(out, flatItem{row: scanner.DirRow(t.Root)})
	var walk func(d *scanner.Dir, depth int)
	walk = func(d *scanner.Dir, depth int) {
		ps := u.sizeOf(scanner.DirRow(d))
		for _, r := range t.Children(d, u.sortBy, u.sortDesc, u.settings.ShowFiles) {
			out = append(out, flatItem{row: r, depth: depth, parentSize: ps})
			if r.IsDir && u.expanded[r.Key] {
				if cd, _, ok := t.Resolve(r.Key); ok {
					walk(cd, depth+1)
				}
			}
		}
	}
	if u.expanded["d0"] {
		walk(t.Root, 1)
	}
	u.flat = out
	// forget expanded folders that no longer exist
	for k := range u.expanded {
		if _, _, ok := t.Resolve(k); !ok {
			delete(u.expanded, k)
		}
	}
	if u.wantReveal != "" {
		k := u.wantReveal
		u.wantReveal = ""
		if i := u.indexOf(k); i >= 0 {
			u.selected = k
			u.treeList.EnsureVisible(i, u.rowH, u.treeRect.Dy()-u.headH)
		}
	}
}

func (u *UI) indexOf(key string) int {
	for i, f := range u.flat {
		if f.row.Key == key {
			return i
		}
	}
	return -1
}

func (u *UI) toggle(key string, open bool) {
	if open {
		u.expanded[key] = true
	} else {
		delete(u.expanded, key)
	}
	u.rebuildFlat()
}

// select marks a row and points the side panel at its folder.
func (u *UI) selectKey(key string, scroll bool) {
	u.selected = key
	if scroll {
		if i := u.indexOf(key); i >= 0 {
			u.treeList.EnsureVisible(i, u.rowH, u.treeRect.Dy()-u.headH)
		}
	}
	u.setFocus(u.dirKeyOf(key))
	u.mapView.selKey = key
}

func (u *UI) setFocus(dir string) {
	if dir != u.focusDir {
		u.focusDir = dir
		u.topFilter = ""
		u.requestSide()
	}
}

// reveal expands the tree down to key and selects it.
func (u *UI) reveal(key string, moveFocus bool) {
	t := u.tree
	if t == nil {
		return
	}
	d, idx, ok := t.Resolve(key)
	if !ok {
		return
	}
	if idx < 0 {
		d = d.Parent
	}
	for p := d; p != nil; p = p.Parent {
		u.expanded[scanner.DirRow(p).Key] = true
	}
	u.wantReveal = key
	u.rebuildFlat()
	u.selected = key
	u.mapView.selKey = key
	if moveFocus {
		u.setFocus(u.dirKeyOf(key))
	}
	u.focusReq = true
}

func (u *UI) parentKey(i int) string {
	if i <= 0 || i >= len(u.flat) {
		return ""
	}
	dep := u.flat[i].depth
	for j := i - 1; j >= 0; j-- {
		if u.flat[j].depth < dep {
			return u.flat[j].row.Key
		}
	}
	return ""
}

// ---------------------------------------------------------------- layout

type column struct {
	id, label string
	w         float32 // dp, 0 = flexible
}

var treeColumns = []column{
	{"name", "colName", 0}, {"size", "colSize", 90}, {"alloc", "colAlloc", 90}, {"pct", "colPct", 136},
	{"files", "colFiles", 78}, {"dirs", "colDirs", 72}, {"mtime", "colMtime", 132},
}

// visibleColumns drops optional columns when the pane gets narrow.
func (u *UI) visibleColumns(gtx layout.Context, width int) ([]column, []int) {
	cols := append([]column(nil), treeColumns...)
	dropOrder := []string{"dirs", "alloc", "mtime", "files"}
	for {
		sum := 0
		for _, c := range cols {
			sum += u.dp(gtx, c.w)
		}
		if width-sum >= u.dp(gtx, 220) || len(dropOrder) == 0 {
			break
		}
		drop := dropOrder[0]
		dropOrder = dropOrder[1:]
		for i, c := range cols {
			if c.id == drop {
				cols = append(cols[:i], cols[i+1:]...)
				break
			}
		}
	}
	xs := make([]int, len(cols)+1)
	fixed := 0
	for _, c := range cols {
		fixed += u.dp(gtx, c.w)
	}
	x := 0
	for i, c := range cols {
		xs[i] = x
		if c.w == 0 {
			x += width - fixed - u.dp(gtx, 10)
		} else {
			x += u.dp(gtx, c.w)
		}
	}
	xs[len(cols)] = x
	return cols, xs
}

func (u *UI) treeView(gtx layout.Context, r image.Rectangle) {
	u.treeRect = r
	u.rowH = u.dp(gtx, 26)
	u.headH = u.dp(gtx, 30)
	fill(gtx, r, u.pal.Surface)
	within(gtx, r, func(gtx layout.Context) {
		size := r.Size()
		cols, xs := u.visibleColumns(gtx, size.X)
		u.treeHeader(gtx, image.Rect(0, 0, size.X, u.headH), cols, xs)
		lr := image.Rect(0, u.headH, size.X, size.Y)
		within(gtx, lr, func(gtx layout.Context) {
			u.treeRows(gtx, lr.Size(), cols, xs, r.Min.Add(lr.Min))
		})
	})
}

func (u *UI) treeHeader(gtx layout.Context, r image.Rectangle, cols []column, xs []int) {
	fill(gtx, image.Rect(r.Min.X, r.Max.Y-1, r.Max.X, r.Max.Y), u.pal.Line)
	sortKeyOf := func(id string) string {
		if id == "pct" {
			return u.settings.SizeMode
		}
		return id
	}
	for _, e := range pointerEvents(gtx, &u.treeHead, pointer.Press) {
		if e.Buttons != pointer.ButtonPrimary {
			continue
		}
		for i, c := range cols {
			if int(e.Position.X) >= xs[i] && int(e.Position.X) < xs[i+1] {
				k := sortKeyOf(c.id)
				if u.sortBy == k {
					u.sortDesc = !u.sortDesc
				} else {
					u.sortBy, u.sortDesc = k, k != "name"
				}
				u.flatDirty = true
			}
		}
	}
	pad := u.dp(gtx, 10)
	for i, c := range cols {
		cr := image.Rect(xs[i]+pad, r.Min.Y, xs[i+1]-pad, r.Max.Y)
		sorted := u.sortBy == sortKeyOf(c.id)
		col, w := u.pal.Muted, font.Normal
		if sorted {
			col, w = u.pal.Ink, font.SemiBold
		}
		lbl := u.t(c.label)
		align := text.End
		if c.id == "name" {
			align = text.Start
			cr.Min.X = xs[i] + u.dp(gtx, 6)
		}
		if sorted && c.id != "pct" {
			// arrow next to the label
			is := u.dp(gtx, 16)
			icn := ic.arrowDown
			if !u.sortDesc {
				icn = ic.arrowUp
			}
			cy := r.Min.Y + r.Dy()/2
			if align == text.End {
				u.icon(gtx, icn, image.Rect(cr.Max.X-is, cy-is/2, cr.Max.X, cy+is/2), col)
				cr.Max.X -= is
			} else {
				tw := u.textWidth(gtx, lbl, 12, w)
				u.icon(gtx, icn, image.Rect(cr.Min.X+tw, cy-is/2, cr.Min.X+tw+is, cy+is/2), col)
			}
		}
		u.text(gtx, cr, lbl, 12, col, w, align)
	}
	hitArea(gtx, r, &u.treeHead, pointer.CursorPointer)
}

func (u *UI) treeRows(gtx layout.Context, size image.Point, cols []column, xs []int, origin image.Point) {
	n := len(u.flat)
	rowH := u.rowH
	indent := u.dp(gtx, 16)
	twistW := u.dp(gtx, 18)

	// keyboard focus
	if u.focusReq {
		gtx.Execute(key.FocusCmd{Tag: &u.treeList.tag})
		u.focusReq = false
	}
	u.treeKeys(gtx, size.Y)

	for _, e := range u.treeList.Update(gtx, n, rowH, size) {
		if e.Kind != pointer.Press || e.Row < 0 {
			continue
		}
		gtx.Execute(key.FocusCmd{Tag: &u.treeList.tag})
		it := u.flat[e.Row]
		k := it.row.Key
		tx := u.dp(gtx, 4) + it.depth*indent
		onTwisty := int(e.Pos.X) >= tx && int(e.Pos.X) < tx+twistW && it.row.IsDir && it.row.HasKids
		switch {
		case e.Buttons == pointer.ButtonSecondary:
			u.selectKey(k, false)
			u.itemMenu(origin.Add(e.Pos.Round()), k, "tree")
		case onTwisty:
			u.toggle(k, !u.expanded[k])
		default:
			dbl := u.lastPress.row == e.Row && e.Time-u.lastPress.at < 450*time.Millisecond
			u.lastPress.row, u.lastPress.at = e.Row, e.Time
			u.selectKey(k, false)
			if dbl {
				u.lastPress.at = 0
				if it.row.IsDir {
					u.toggle(k, !u.expanded[k])
				} else {
					u.openItem(k)
				}
			}
		}
	}
	n = len(u.flat)

	pad := u.dp(gtx, 10)
	focused := gtx.Focused(&u.treeList.tag)
	u.treeList.Layout(gtx, u, size, n, rowH, func(gtx layout.Context, i int, rr image.Rectangle) {
		it := u.flat[i]
		r := it.row
		if r.Key == u.selected {
			fill(gtx, rr, u.pal.Sel)
			if focused {
				fill(gtx, image.Rect(rr.Min.X, rr.Min.Y, rr.Min.X+u.dp(gtx, 2), rr.Max.Y), u.pal.Accent)
			}
		} else if i == u.treeList.Hover {
			fill(gtx, rr, u.pal.Hover)
		}
		fill(gtx, image.Rect(rr.Min.X, rr.Max.Y-1, rr.Max.X, rr.Max.Y), u.pal.LineSoft)
		s := u.sizeOf(r)
		pct := 100.0
		if it.depth > 0 {
			pct = 0
			if it.parentSize > 0 {
				pct = float64(s) * 100 / float64(it.parentSize)
			}
		}
		for ci, c := range cols {
			cr := image.Rect(xs[ci]+pad, rr.Min.Y, xs[ci+1]-pad, rr.Max.Y)
			switch c.id {
			case "name":
				x := xs[ci] + u.dp(gtx, 4) + it.depth*indent
				is := u.dp(gtx, 14)
				cy := rr.Min.Y + rowH/2
				if r.IsDir && r.HasKids && !r.Linked {
					icn := ic.chevron
					if u.expanded[r.Key] {
						icn = ic.chevronDown
					}
					u.icon(gtx, icn, image.Rect(x+2, cy-is/2, x+2+is, cy+is/2), u.pal.Muted)
				}
				x += twistW
				fs := u.dp(gtx, 16)
				if r.IsDir {
					u.icon(gtx, ic.folder, image.Rect(x, cy-fs/2, x+fs, cy+fs/2), u.pal.Folder)
				} else {
					u.icon(gtx, ic.file, image.Rect(x, cy-fs/2, x+fs, cy+fs/2), u.pal.Faint)
				}
				x += fs + u.dp(gtx, 6)
				name := r.Name
				if it.depth == 0 {
					name = u.tree.RootPath
				}
				end := xs[ci+1] - pad
				badge, bcol := "", u.pal.Warn
				switch {
				case r.Err != "":
					badge = u.t("errOther")
					if r.Err == "denied" {
						badge = u.t("errDenied")
					}
				case r.Linked:
					badge, bcol = u.t("linked"), u.pal.Faint
				}
				bw := 0
				if badge != "" {
					bw = u.textWidth(gtx, badge, 11, font.Normal) + u.dp(gtx, 8)
				} else if r.Online {
					bw = u.dp(gtx, 20)
				} else if r.IsDir && !r.Done {
					bw = u.dp(gtx, 14)
				}
				nw := u.text(gtx, image.Rect(x, rr.Min.Y, end-bw, rr.Max.Y), name, 13, u.pal.Ink, font.Normal, text.Start)
				bx := x + nw + u.dp(gtx, 6)
				switch {
				case badge != "":
					u.text(gtx, image.Rect(bx, rr.Min.Y, end, rr.Max.Y), badge, 11, bcol, font.Normal, text.Start)
				case r.Online:
					u.icon(gtx, ic.cloud, image.Rect(bx, cy-u.dp(gtx, 7), bx+u.dp(gtx, 14), cy+u.dp(gtx, 7)), u.pal.Accent)
				case r.IsDir && !r.Done:
					dot := u.dp(gtx, 6)
					fillRR(gtx, image.Rect(bx, cy-dot/2, bx+dot, cy+dot/2), dot/2, u.pal.Accent)
				}
			case "size":
				u.text(gtx, cr, u.size(r.Size), 13, u.pal.Ink, font.Normal, text.End)
			case "alloc":
				u.text(gtx, cr, u.size(r.Alloc), 13, u.pal.Muted, font.Normal, text.End)
			case "pct":
				numW := u.dp(gtx, 46)
				u.text(gtx, image.Rect(cr.Max.X-numW, rr.Min.Y, cr.Max.X, rr.Max.Y), u.pct(pct), 12, u.pal.Muted, font.Normal, text.End)
				bh := u.dp(gtx, 8)
				cy := rr.Min.Y + rowH/2
				bar := image.Rect(cr.Min.X, cy-bh/2, cr.Max.X-numW-u.dp(gtx, 8), cy+bh/2)
				if bar.Dx() > 10 {
					fillRR(gtx, bar, u.dp(gtx, 2), u.pal.BarTrack)
					p := pct
					if p > 100 {
						p = 100
					}
					col := u.pal.Accent
					if !r.IsDir {
						col = u.pal.Faint
					}
					fw := int(float64(bar.Dx()) * p / 100)
					if fw > 0 {
						fillRR(gtx, image.Rect(bar.Min.X, bar.Min.Y, bar.Min.X+fw, bar.Max.Y), u.dp(gtx, 2), col)
					}
				}
			case "files":
				if r.IsDir {
					u.text(gtx, cr, u.num(r.Files), 13, u.pal.Muted, font.Normal, text.End)
				}
			case "dirs":
				if r.IsDir {
					u.text(gtx, cr, u.num(r.Dirs), 13, u.pal.Muted, font.Normal, text.End)
				}
			case "mtime":
				u.text(gtx, cr, u.date(r.MTime), 13, u.pal.Muted, font.Normal, text.End)
			}
		}
	})
}

func (u *UI) treeKeys(gtx layout.Context, viewH int) {
	tag := &u.treeList.tag
	names := []key.Name{key.NameUpArrow, key.NameDownArrow, key.NamePageUp, key.NamePageDown, key.NameHome, key.NameEnd,
		key.NameLeftArrow, key.NameRightArrow, key.NameReturn, key.NameEnter, key.NameDeleteForward}
	for {
		var ev interface{}
		var ok bool
		fs := []event.Filter{key.FocusFilter{Target: tag}, key.Filter{Focus: tag, Name: "C", Required: key.ModShortcut}}
		for _, n := range names {
			fs = append(fs, key.Filter{Focus: tag, Name: n})
		}
		ev, ok = gtx.Event(fs...)
		if !ok {
			break
		}
		e, isKey := ev.(key.Event)
		if !isKey || e.State != key.Press {
			continue
		}
		i := u.indexOf(u.selected)
		if i < 0 {
			i = 0
		}
		page := viewH/u.rowH - 1
		goTo := func(j int) {
			if len(u.flat) == 0 {
				return
			}
			if j < 0 {
				j = 0
			}
			if j >= len(u.flat) {
				j = len(u.flat) - 1
			}
			u.selectKey(u.flat[j].row.Key, true)
		}
		if len(u.flat) == 0 {
			continue
		}
		it := u.flat[i]
		switch e.Name {
		case key.NameDownArrow:
			goTo(i + 1)
		case key.NameUpArrow:
			goTo(i - 1)
		case key.NamePageDown:
			goTo(i + page)
		case key.NamePageUp:
			goTo(i - page)
		case key.NameHome:
			goTo(0)
		case key.NameEnd:
			goTo(len(u.flat) - 1)
		case key.NameRightArrow:
			if it.row.IsDir && it.row.HasKids {
				if !u.expanded[it.row.Key] {
					u.toggle(it.row.Key, true)
				} else {
					goTo(i + 1)
				}
			}
		case key.NameLeftArrow:
			if it.row.IsDir && u.expanded[it.row.Key] {
				u.toggle(it.row.Key, false)
			} else if p := u.parentKey(i); p != "" {
				u.selectKey(p, true)
			}
		case key.NameReturn, key.NameEnter:
			if it.row.IsDir {
				u.toggle(it.row.Key, !u.expanded[it.row.Key])
			} else {
				u.openItem(it.row.Key)
			}
		case key.NameDeleteForward:
			u.askDelete(it.row.Key)
		case "C":
			u.copyPath(it.row.Key)
		}
	}
}

var _ = strings.HasPrefix
