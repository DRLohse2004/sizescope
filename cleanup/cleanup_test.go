package cleanup

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func write(t *testing.T, p string, n int, age time.Duration) {
	t.Helper()
	os.MkdirAll(filepath.Dir(p), 0o755)
	if err := os.WriteFile(p, make([]byte, n), 0o644); err != nil {
		t.Fatal(err)
	}
	if age > 0 {
		ts := time.Now().Add(-age)
		os.Chtimes(p, ts, ts)
	}
}

func TestRules(t *testing.T) {
	root := t.TempDir()
	day := 24 * time.Hour
	write(t, filepath.Join(root, "u1", "Temp", "old.tmp"), 100, 3*day)
	write(t, filepath.Join(root, "u1", "Temp", "sub", "old2.tmp"), 50, 3*day)
	write(t, filepath.Join(root, "u1", "Temp", "new.tmp"), 7, 0)
	write(t, filepath.Join(root, "u2", "Temp", "keep.log"), 30, 3*day)
	write(t, filepath.Join(root, "u2", "Temp", "x.tmp"), 20, 3*day)
	os.Setenv("SS_TEST_ROOT", root)

	d := &Definition{ID: "t", Name: Text{"": "test"}, Targets: []Target{{
		Path: `%SS_TEST_ROOT%/*/Temp`, MinAgeDays: 1, Exclude: []string{"*.log"},
	}}}
	pre := Run(context.Background(), []*Definition{d}, true, nil)[0]
	if pre.Files != 3 || pre.Bytes != 170 || len(pre.Locations) != 2 {
		t.Fatalf("preview %+v", pre)
	}
	if _, err := os.Stat(filepath.Join(root, "u1", "Temp", "old.tmp")); err != nil {
		t.Fatal("preview deleted a file")
	}
	res := Run(context.Background(), []*Definition{d}, false, nil)[0]
	if res.Files != 3 || res.Bytes != 170 {
		t.Fatalf("run %+v", res)
	}
	for _, keep := range []string{"u1/Temp/new.tmp", "u2/Temp/keep.log", "u1/Temp"} {
		if _, err := os.Stat(filepath.Join(root, keep)); err != nil {
			t.Fatalf("%s removed", keep)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "u1", "Temp", "sub")); err == nil {
		t.Fatal("empty sub folder not removed")
	}
}

func TestSafety(t *testing.T) {
	os.Unsetenv("SS_UNSET_VAR")
	if _, ok := ExpandPath(`%SS_UNSET_VAR%\Temp`); ok {
		t.Fatal("unset variable accepted")
	}
	if safeBase("/") || safeBase("/home") || safeBase(`relative/x`) {
		t.Fatal("unsafe base accepted")
	}
	if !safeBase("/tmp/x") {
		t.Fatal("safe base refused")
	}
	// a shallow recursive target must be refused
	d := &Definition{ID: "x", Name: Text{"": "x"}, Targets: []Target{{Path: "/tmp"}}}
	r := Run(context.Background(), []*Definition{d}, true, nil)[0]
	if r.Files != 0 || len(r.Locations) != 1 || r.Locations[0].Note == "" {
		t.Fatalf("shallow target not refused: %+v", r)
	}
}

func TestLoadAndImport(t *testing.T) {
	dir := t.TempDir()
	js := filepath.Join(dir, "src.json")
	os.WriteFile(js, []byte(`{"name":{"de":"Mein Cache","en":"My cache"},"admin":true,"targets":[{"path":"%TEMP%\\foo","minAgeDays":2}]}`), 0o644)
	ps := filepath.Join(dir, "clean.ps1")
	os.WriteFile(ps, []byte("# Name-de: Skript\n# Name: Script\n# Admin: true\n# Preview: true\nWrite-Output 'SIZESCOPE|C:\\x|123|4'\n"), 0o644)
	lib := filepath.Join(dir, "lib")
	for _, f := range []string{js, ps} {
		if _, err := Import(f, lib); err != nil {
			t.Fatal(err)
		}
	}
	defs, errs := LoadDir(lib)
	if len(errs) != 0 || len(defs) != 2 {
		t.Fatalf("defs %d errs %v", len(defs), errs)
	}
	var sawJSON, sawPS bool
	for _, d := range defs {
		if d.Script != "" {
			sawPS = d.Name.Get("de") == "Skript" && d.Admin && d.Preview
		} else {
			sawJSON = d.Name.Get("en") == "My cache" && d.Admin && d.Targets[0].MinAgeDays == 2
		}
	}
	if !sawJSON || !sawPS {
		t.Fatalf("parsed wrong: %+v %+v", defs[0], defs[1])
	}
	if _, err := Import(filepath.Join(dir, "x.txt"), lib); err == nil {
		t.Fatal("txt import accepted")
	}
	if got := parseReport("noise\nSIZESCOPE|C:\\a|10|2\nSIZESCOPE|D:\\b|5\n"); len(got) != 2 || got[0].Bytes != 10 || got[1].Files != 0 {
		t.Fatalf("report %+v", got)
	}
	if n := len(Builtin()); n < 10 {
		t.Fatalf("builtins %d", n)
	}
}
