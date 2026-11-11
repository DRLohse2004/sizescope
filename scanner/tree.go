// Package scanner builds an in-memory size tree of a directory hierarchy.
// Scanning is parallel and results can be queried while a scan is running.
package scanner

import (
	"context"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// File is a regular file inside a directory.
type File struct {
	Name  string
	Size  int64
	Alloc int64
	MTime int64 // unix seconds
}

// Dir is a directory node. Totals cover the whole subtree.
type Dir struct {
	ID     uint32
	Name   string
	Parent *Dir

	mu    sync.RWMutex
	dirs  []*Dir
	files []File

	size, alloc, nFiles, nDirs, mtime atomic.Int64
	pending                           atomic.Int32
	done                              atomic.Bool
	removed                           atomic.Bool
	err                               string // "", "denied", "error"
	Linked                            bool   // reparse point (junction/symlink), not followed
}

// Dirs returns a snapshot of the subdirectories.
func (d *Dir) Dirs() []*Dir {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.dirs
}

// Err returns "", "denied" or "error" for folders that could not be read.
func (d *Dir) Err() string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.err
}

// Files returns a snapshot of the files.
func (d *Dir) Files() []File {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.files
}

func (d *Dir) Size() int64      { return d.size.Load() }
func (d *Dir) Alloc() int64     { return d.alloc.Load() }
func (d *Dir) FileCount() int64 { return d.nFiles.Load() }
func (d *Dir) DirCount() int64  { return d.nDirs.Load() }
func (d *Dir) MTime() int64     { return d.mtime.Load() }
func (d *Dir) Done() bool       { return d.done.Load() }

// Entry is what a platform lister reports per directory entry.
type Entry struct {
	Name   string
	IsDir  bool
	Linked bool // reparse point / symlink that must not be followed
	Online bool // cloud file that is only stored online (no local data)
	Size   int64
	Alloc  int64
	MTime  int64
}

// Options control a scan.
type Options struct {
	Exclude    []string // glob patterns matched against entry names (case-insensitive)
	Workers    int
	CloudFetch bool // let cloud providers download folder listings that are only online (slow)
	SkipOnline bool // leave out files that are only stored online (OneDrive "available online")
}

// Tree is the result of one scan.
type Tree struct {
	ID       int64
	RootPath string
	Root     *Dir
	Started  time.Time

	mu       sync.RWMutex
	nodes    []*Dir
	finished atomic.Int64 // unix ms, 0 while running

	files, dirs, bytes, errors atomic.Int64
	onlineFiles, onlineBytes   atomic.Int64
	current                    atomic.Value // string

	opts     Options
	cluster  int64
	boundary *Dir // completion does not propagate above this node (rescan)
	cancel   context.CancelFunc
	ctx      context.Context
	sem      chan struct{}
	wg       sync.WaitGroup
}

var treeSeq atomic.Int64

// NewTree prepares a scan of root without starting it.
func NewTree(root string, opts Options) *Tree {
	root = cleanRoot(root)
	if opts.Workers <= 0 {
		opts.Workers = runtime.NumCPU() * 2
		if opts.Workers < 4 {
			opts.Workers = 4
		}
		if opts.Workers > 32 {
			opts.Workers = 32
		}
	}
	t := &Tree{ID: treeSeq.Add(1), RootPath: root, opts: opts}
	t.Root = t.newDir(root, nil)
	t.current.Store(root)
	return t
}

func cleanRoot(p string) string {
	p = filepath.Clean(p)
	// "C:" -> "C:\"
	if len(p) == 2 && p[1] == ':' {
		p += string(filepath.Separator)
	}
	return p
}

func (t *Tree) newDir(name string, parent *Dir) *Dir {
	d := &Dir{Name: name, Parent: parent}
	t.mu.Lock()
	d.ID = uint32(len(t.nodes))
	t.nodes = append(t.nodes, d)
	t.mu.Unlock()
	return d
}

// Node returns a directory by id (nil if unknown or removed).
func (t *Tree) Node(id uint32) *Dir {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if int(id) >= len(t.nodes) {
		return nil
	}
	d := t.nodes[id]
	if d.removed.Load() {
		return nil
	}
	return d
}

// Start launches the scan in the background.
func (t *Tree) Start(parent context.Context) {
	t.ctx, t.cancel = context.WithCancel(parent)
	t.sem = make(chan struct{}, t.opts.Workers)
	t.Started = time.Now()
	t.cluster = clusterSize(t.RootPath)
	go func() {
		t.walk(t.Root, t.RootPath)
		t.wg.Wait()
		t.finished.Store(time.Now().UnixMilli())
	}()
}

// Cancel stops a running scan. Partial results stay available.
func (t *Tree) Cancel() {
	if t.cancel != nil {
		t.cancel()
	}
}

// SkipOnline reports whether online-only files are left out of this scan.
func (t *Tree) SkipOnline() bool { return t.opts.SkipOnline }

// Running reports whether the scan is still in progress.
func (t *Tree) Running() bool { return t.finished.Load() == 0 }

// Wait blocks until the scan is finished (for tests / CLI use).
func (t *Tree) Wait() {
	for t.Running() {
		time.Sleep(10 * time.Millisecond)
	}
}

// Stats is a progress snapshot.
type Stats struct {
	Files, Dirs, Bytes, Errors int64
	OnlineFiles, OnlineBytes   int64 // online-only cloud files seen (counted or skipped)
	Elapsed                    time.Duration
	Current                    string
	Canceled                   bool
}

func (t *Tree) Stats() Stats {
	end := time.Now()
	if f := t.finished.Load(); f != 0 {
		end = time.UnixMilli(f)
	}
	s := Stats{
		Files: t.files.Load(), Dirs: t.dirs.Load(), Bytes: t.bytes.Load(), Errors: t.errors.Load(),
		OnlineFiles: t.onlineFiles.Load(), OnlineBytes: t.onlineBytes.Load(),
		Elapsed: end.Sub(t.Started),
	}
	s.Current, _ = t.current.Load().(string)
	if t.ctx != nil && t.ctx.Err() != nil {
		s.Canceled = true
	}
	return s
}

func (t *Tree) excluded(name string) bool {
	if len(t.opts.Exclude) == 0 {
		return false
	}
	ln := strings.ToLower(name)
	for _, p := range t.opts.Exclude {
		if ok, _ := filepath.Match(strings.ToLower(p), ln); ok {
			return true
		}
	}
	return false
}

func (t *Tree) walk(d *Dir, path string) {
	if t.ctx.Err() != nil {
		t.finish(d)
		return
	}
	t.current.Store(path)
	entries, err := listDir(path, t.cluster, t.opts.CloudFetch)
	errCode := ""
	if err != nil {
		errCode = classifyErr(err)
		t.errors.Add(1)
	}

	var files []File
	var subs []*Dir
	var own, ownAlloc, newest, onlineN, onlineB int64
	for _, e := range entries {
		if t.excluded(e.Name) {
			continue
		}
		if e.IsDir {
			c := t.newDir(e.Name, d)
			if e.Linked {
				c.Linked = true
				c.done.Store(true)
			}
			c.mtime.Store(e.MTime)
			subs = append(subs, c)
			continue
		}
		if e.Online {
			onlineN++
			onlineB += e.Size
			if t.opts.SkipOnline {
				continue
			}
		}
		files = append(files, File{Name: e.Name, Size: e.Size, Alloc: e.Alloc, MTime: e.MTime})
		own += e.Size
		ownAlloc += e.Alloc
		if e.MTime > newest {
			newest = e.MTime
		}
	}

	d.mu.Lock()
	d.files = files
	d.dirs = subs
	d.err = errCode
	d.mu.Unlock()

	t.files.Add(int64(len(files)))
	t.dirs.Add(int64(len(subs)))
	t.bytes.Add(own)
	if onlineN > 0 {
		t.onlineFiles.Add(onlineN)
		t.onlineBytes.Add(onlineB)
	}
	addUp(d, own, ownAlloc, int64(len(files)), int64(len(subs)), newest)

	toWalk := make([]*Dir, 0, len(subs))
	for _, c := range subs {
		if !c.Linked {
			toWalk = append(toWalk, c)
		}
	}
	d.pending.Store(int32(len(toWalk) + 1))
	for _, c := range toWalk {
		cp := joinPath(path, c.Name)
		select {
		case t.sem <- struct{}{}:
			t.wg.Add(1)
			go func(c *Dir, cp string) {
				defer t.wg.Done()
				t.walk(c, cp)
				<-t.sem
			}(c, cp)
		default:
			t.walk(c, cp)
		}
	}
	t.finish(d)
}

// finish marks one unit of work of d as complete and propagates completion upwards.
func (t *Tree) finish(d *Dir) {
	for d != nil {
		if d.pending.Add(-1) > 0 {
			return
		}
		d.done.Store(true)
		if d == t.boundary {
			return
		}
		d = d.Parent
	}
}

func addUp(d *Dir, size, alloc, files, dirs, mtime int64) {
	for p := d; p != nil; p = p.Parent {
		p.size.Add(size)
		p.alloc.Add(alloc)
		p.nFiles.Add(files)
		p.nDirs.Add(dirs)
		for {
			cur := p.mtime.Load()
			if mtime <= cur || p.mtime.CompareAndSwap(cur, mtime) {
				break
			}
		}
	}
}

func joinPath(dir, name string) string {
	if strings.HasSuffix(dir, string(filepath.Separator)) {
		return dir + name
	}
	return dir + string(filepath.Separator) + name
}

// PathOf returns the full path of a directory.
func (t *Tree) PathOf(d *Dir) string {
	var parts []string
	for p := d; p != nil && p != t.Root; p = p.Parent {
		parts = append(parts, p.Name)
	}
	path := t.RootPath
	for i := len(parts) - 1; i >= 0; i-- {
		path = joinPath(path, parts[i])
	}
	return path
}

// RemoveDir detaches a directory (after deleting it on disk) and fixes the totals.
func (t *Tree) RemoveDir(d *Dir) {
	if d == nil || d.Parent == nil {
		return
	}
	p := d.Parent
	addUp(p, -d.Size(), -d.Alloc(), -d.FileCount(), -d.DirCount()-1, 0)
	p.mu.Lock()
	out := p.dirs[:0:0]
	for _, c := range p.dirs {
		if c != d {
			out = append(out, c)
		}
	}
	p.dirs = out
	p.mu.Unlock()
	markRemoved(d)
}

func markRemoved(d *Dir) {
	d.removed.Store(true)
	for _, c := range d.Dirs() {
		markRemoved(c)
	}
}

// RemoveFile detaches a file by index from its directory.
func (t *Tree) RemoveFile(d *Dir, idx int) {
	d.mu.Lock()
	if idx < 0 || idx >= len(d.files) {
		d.mu.Unlock()
		return
	}
	f := d.files[idx]
	nf := make([]File, 0, len(d.files)-1)
	nf = append(nf, d.files[:idx]...)
	nf = append(nf, d.files[idx+1:]...)
	d.files = nf
	d.mu.Unlock()
	addUp(d, -f.Size, -f.Alloc, -1, 0, 0)
}

// Rescan rereads a directory subtree in place (only when no scan is running).
func (t *Tree) Rescan(d *Dir) {
	if d == nil {
		return
	}
	if d.Parent != nil {
		addUp(d.Parent, -d.Size(), -d.Alloc(), -d.FileCount(), -d.DirCount(), 0)
	}
	for _, c := range d.Dirs() {
		markRemoved(c)
	}
	d.mu.Lock()
	d.dirs, d.files = nil, nil
	d.mu.Unlock()
	d.size.Store(0)
	d.alloc.Store(0)
	d.nFiles.Store(0)
	d.nDirs.Store(0)
	d.mtime.Store(0)
	d.err = ""
	d.done.Store(false)
	path := t.PathOf(d)
	t.boundary = d
	t.finished.Store(0)
	t.ctx, t.cancel = context.WithCancel(context.Background())
	t.Started = time.Now()
	t.files.Store(0)
	t.dirs.Store(0)
	t.bytes.Store(0)
	t.errors.Store(0)
	t.onlineFiles.Store(0)
	t.onlineBytes.Store(0)
	go func() {
		t.walk(d, path)
		t.wg.Wait()
		t.finished.Store(time.Now().UnixMilli())
	}()
}
