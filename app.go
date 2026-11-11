package main

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"sizescope/scanner"
)

const appVersion = "1.0.0"

// App is bound to the frontend; every exported method is callable from JS.
type App struct {
	ctx      context.Context
	mu       sync.Mutex
	tree     *scanner.Tree
	settings Settings
	startArg string
}

type Drive struct {
	Path  string `json:"path"`
	Label string `json:"label"`
	FS    string `json:"fs"`
	Kind  string `json:"kind"`
	Total int64  `json:"total"`
	Free  int64  `json:"free"`
}

func NewApp() *App {
	a := &App{settings: loadSettings()}
	if len(os.Args) > 1 && !strings.HasPrefix(os.Args[1], "-") {
		a.startArg = os.Args[1]
	}
	return a
}

func (a *App) startup(ctx context.Context) { a.ctx = ctx }

func (a *App) current() *scanner.Tree {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.tree
}

var errNoScan = errors.New("noscan")

func (a *App) dir(key string) (*scanner.Tree, *scanner.Dir, error) {
	t := a.current()
	if t == nil {
		return nil, nil, errNoScan
	}
	if key == "" {
		return t, t.Root, nil
	}
	d, idx, ok := t.Resolve(key)
	if !ok || idx >= 0 {
		return t, nil, errors.New("notfound")
	}
	return t, d, nil
}

// ---- setup -------------------------------------------------------------

type InitInfo struct {
	Settings Settings `json:"settings"`
	Drives   []Drive  `json:"drives"`
	Admin    bool     `json:"admin"`
	Version  string   `json:"version"`
	Platform string   `json:"platform"`
	StartArg string   `json:"startArg"`
}

func (a *App) Init() InitInfo {
	return InitInfo{a.settings, listDrives(), isAdmin(), appVersion, platformName, a.startArg}
}

func (a *App) Drives() []Drive { return listDrives() }

func (a *App) SaveSettings(s Settings) error {
	a.mu.Lock()
	a.settings = s
	a.mu.Unlock()
	return saveSettings(s)
}

func (a *App) PickFolder() (string, error) {
	return runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{})
}

// ---- scanning ----------------------------------------------------------

func (a *App) StartScan(path string) error {
	path = strings.TrimSpace(strings.Trim(path, `"`))
	st, err := os.Stat(path)
	if err != nil || !st.IsDir() {
		return errors.New("notadir")
	}
	a.mu.Lock()
	if a.tree != nil {
		a.tree.Cancel()
	}
	t := scanner.NewTree(path, scanner.Options{Exclude: a.settings.Exclude})
	a.tree = t
	// remember recently scanned folders
	rec := []string{t.RootPath}
	for _, r := range a.settings.Recent {
		if !strings.EqualFold(r, t.RootPath) && len(rec) < 8 {
			rec = append(rec, r)
		}
	}
	a.settings.Recent = rec
	s := a.settings
	a.mu.Unlock()
	_ = saveSettings(s)
	t.Start(context.Background())
	return nil
}

func (a *App) Cancel() {
	if t := a.current(); t != nil {
		t.Cancel()
	}
}

type Status struct {
	Has       bool        `json:"has"`
	ID        int64       `json:"id"`
	Running   bool        `json:"running"`
	Canceled  bool        `json:"canceled"`
	Root      scanner.Row `json:"root"`
	RootPath  string      `json:"rootPath"`
	Files     int64       `json:"files"`
	Dirs      int64       `json:"dirs"`
	Bytes     int64       `json:"bytes"`
	Errors    int64       `json:"errors"`
	ElapsedMs int64       `json:"elapsedMs"`
	Current   string      `json:"current"`
	VolTotal  int64       `json:"volTotal"`
	VolFree   int64       `json:"volFree"`
}

func (a *App) Status() Status {
	t := a.current()
	if t == nil {
		return Status{}
	}
	s := t.Stats()
	tot, free := diskUsage(t.RootPath)
	return Status{
		Has: true, ID: t.ID, Running: t.Running(), Canceled: s.Canceled,
		Root: scanner.DirRow(t.Root), RootPath: t.RootPath,
		Files: s.Files, Dirs: s.Dirs, Bytes: s.Bytes, Errors: s.Errors,
		ElapsedMs: s.Elapsed.Milliseconds(), Current: s.Current, VolTotal: tot, VolFree: free,
	}
}

// ---- queries -----------------------------------------------------------

type ChildList struct {
	Parent scanner.Row   `json:"parent"`
	Rows   []scanner.Row `json:"rows"`
}

func (a *App) Children(key, sortBy string, desc, withFiles bool) (ChildList, error) {
	t, d, err := a.dir(key)
	if err != nil {
		return ChildList{}, err
	}
	return ChildList{scanner.DirRow(d), t.Children(d, sortBy, desc, withFiles)}, nil
}

// ChildrenMany returns the children of several folders in one call (used to
// refresh all expanded folders while a scan is running).
func (a *App) ChildrenMany(keys []string, sortBy string, desc, withFiles bool) map[string]ChildList {
	out := make(map[string]ChildList, len(keys))
	for _, k := range keys {
		if c, err := a.Children(k, sortBy, desc, withFiles); err == nil {
			out[k] = c
		}
	}
	return out
}

func (a *App) Treemap(key string, maxNodes int) (*scanner.MapNode, error) {
	t, d, err := a.dir(key)
	if err != nil {
		return nil, err
	}
	if maxNodes <= 0 {
		maxNodes = 4000
	}
	return t.Treemap(d, maxNodes, 0.00005), nil
}

func (a *App) Extensions(key string) ([]scanner.ExtStat, error) {
	t, d, err := a.dir(key)
	if err != nil {
		return nil, err
	}
	return t.Extensions(d, 300), nil
}

func (a *App) TopFiles(key string, n int, ext string) ([]scanner.FileInfo, error) {
	t, d, err := a.dir(key)
	if err != nil {
		return nil, err
	}
	if n <= 0 {
		n = 200
	}
	return t.TopFiles(d, n, ext), nil
}

func (a *App) Ages(key string) ([]scanner.AgeBucket, error) {
	t, d, err := a.dir(key)
	if err != nil {
		return nil, err
	}
	return t.Ages(d, time.Now()), nil
}

// Ancestors returns the folder keys from the root down to the item's parent,
// so the tree can expand to reveal it.
func (a *App) Ancestors(key string) []string {
	t := a.current()
	if t == nil {
		return nil
	}
	d, idx, ok := t.Resolve(key)
	if !ok {
		return nil
	}
	if idx < 0 {
		d = d.Parent
	}
	var keys []string
	for p := d; p != nil; p = p.Parent {
		keys = append([]string{scanner.DirRow(p).Key}, keys...)
	}
	return keys
}

func (a *App) PathOf(key string) string {
	t := a.current()
	if t == nil {
		return ""
	}
	p, _ := t.KeyPath(key)
	return p
}

// ---- actions -----------------------------------------------------------

func (a *App) ShowInExplorer(key string) error {
	p := a.PathOf(key)
	if p == "" {
		return errors.New("notfound")
	}
	return showInExplorer(p, strings.HasPrefix(key, "d"))
}

func (a *App) OpenItem(key string) error {
	p := a.PathOf(key)
	if p == "" {
		return errors.New("notfound")
	}
	return openItem(p)
}

func (a *App) CopyText(text string) error { return runtime.ClipboardSetText(a.ctx, text) }

func (a *App) Delete(key string) error {
	t := a.current()
	if t == nil {
		return errNoScan
	}
	if t.Running() {
		return errors.New("busy")
	}
	d, idx, ok := t.Resolve(key)
	if !ok {
		return errors.New("notfound")
	}
	if idx < 0 && d == t.Root {
		return errors.New("root")
	}
	p, _ := t.KeyPath(key)
	if err := moveToRecycleBin(p); err != nil {
		return err
	}
	if idx >= 0 {
		t.RemoveFile(d, idx)
	} else {
		t.RemoveDir(d)
	}
	return nil
}

func (a *App) Rescan(key string) error {
	t, d, err := a.dir(key)
	if err != nil {
		return err
	}
	if t.Running() {
		return errors.New("busy")
	}
	t.Rescan(d)
	return nil
}

// Export writes a CSV report of a folder; returns the file path ("" if cancelled).
func (a *App) Export(key string) (string, error) {
	t, d, err := a.dir(key)
	if err != nil {
		return "", err
	}
	a.mu.Lock()
	s := a.settings
	a.mu.Unlock()
	name := strings.NewReplacer(`\`, "_", ":", "", "/", "_").Replace(d.Name)
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		DefaultFilename: "SizeScope_" + strings.Trim(name, "_") + "_" + time.Now().Format("2006-01-02") + ".csv",
		Filters:         []runtime.FileFilter{{DisplayName: "CSV", Pattern: "*.csv"}},
	})
	if err != nil || path == "" {
		return "", err
	}
	f, err := os.Create(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	f.WriteString("\xEF\xBB\xBF") // BOM so Excel detects UTF-8
	sep := ';'
	if s.CsvSeparator == "," {
		sep = ','
	} else if s.CsvSeparator == "tab" {
		sep = '\t'
	}
	return path, t.ExportCSV(f, d, s.ExportDepth, s.ExportFiles, sep)
}

func (a *App) IsAdmin() bool { return isAdmin() }

func (a *App) RestartAsAdmin() error {
	args := []string{}
	if t := a.current(); t != nil {
		args = append(args, t.RootPath)
	}
	if err := restartAsAdmin(args); err != nil {
		return err
	}
	runtime.Quit(a.ctx)
	return nil
}
