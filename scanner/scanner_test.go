package scanner

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func mk(t *testing.T, root string, files map[string]int) {
	for p, n := range files {
		full := filepath.Join(root, p)
		os.MkdirAll(filepath.Dir(full), 0o755)
		if err := os.WriteFile(full, make([]byte, n), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestScan(t *testing.T) {
	root := t.TempDir()
	mk(t, root, map[string]int{
		"a/x.bin": 1000, "a/b/y.txt": 200, "a/b/c/z.txt": 300,
		"d/big.iso": 5000, "top.log": 10, "skip/me.bin": 99999,
	})
	os.Symlink(filepath.Join(root, "a"), filepath.Join(root, "link"))
	tr := NewTree(root, Options{Exclude: []string{"SKIP"}})
	tr.Start(context.Background())
	tr.Wait()
	r := tr.Root
	if r.Size() != 6510 || r.FileCount() != 5 {
		t.Fatalf("size %d files %d", r.Size(), r.FileCount())
	}
	if !r.Done() {
		t.Fatal("root not done")
	}
	if r.DirCount() != 5 { // a, a/b, a/b/c, d, link
		t.Fatalf("dirs %d", r.DirCount())
	}
	rows := tr.Children(r, "size", true, true)
	if rows[0].Name != "d" || rows[1].Name != "a" || rows[1].Size != 1500 {
		t.Fatalf("rows %+v", rows)
	}
	top := tr.TopFiles(r, 2, "")
	if len(top) != 2 || top[0].Name != "big.iso" || top[1].Name != "x.bin" {
		t.Fatalf("top %+v", top)
	}
	ex := tr.Extensions(r, 0)
	if ex[0].Ext != "iso" || ex[0].Size != 5000 {
		t.Fatalf("ext %+v", ex)
	}
	m := tr.Treemap(r, 1000, 0.0001)
	if m.Size != 6510 || len(m.Kids) == 0 {
		t.Fatalf("map %+v", m)
	}
	ages := tr.Ages(r, time.Now())
	if ages[0].Count != 5 {
		t.Fatalf("ages %+v", ages)
	}
	var buf bytes.Buffer
	if err := tr.ExportCSV(&buf, r, 0, true, ';'); err != nil {
		t.Fatal(err)
	}
	if bytes.Count(buf.Bytes(), []byte("\n")) != 1+6+5 {
		t.Fatalf("csv:\n%s", buf.String())
	}
	// remove + rescan
	a, _, _ := tr.Resolve(rows[1].Key)
	p, _ := tr.KeyPath(rows[1].Key)
	if p != filepath.Join(root, "a") {
		t.Fatalf("path %s", p)
	}
	tr.RemoveDir(a)
	if r.Size() != 5010 || r.DirCount() != 2 {
		t.Fatalf("after remove %d %d", r.Size(), r.DirCount())
	}
	os.WriteFile(filepath.Join(root, "d", "new.bin"), make([]byte, 500), 0o644)
	d, _, _ := tr.Resolve(rows[0].Key)
	tr.Rescan(d)
	tr.Wait()
	if r.Size() != 5510 || d.Size() != 5500 || !d.Done() {
		t.Fatalf("after rescan %d %d", r.Size(), d.Size())
	}
}
