package cleanup

import "testing"

func TestExamples(t *testing.T) {
	for _, f := range []string{"../examples/cleanups/teams-cache.json", "../examples/cleanups/old-downloads.ps1"} {
		d, err := LoadFile(f)
		if err != nil || len(d) != 1 {
			t.Fatal(f, err)
		}
		t.Log(d[0].Name.Get("de"), d[0].Preview, len(d[0].Targets))
	}
}
