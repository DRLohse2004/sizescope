//go:build !windows

package scanner

import (
	"errors"
	"io/fs"
	"os"
	"syscall"
)

func listDir(path string, cluster int64, _ bool) ([]Entry, error) {
	des, err := os.ReadDir(path)
	out := make([]Entry, 0, len(des))
	for _, de := range des {
		info, ierr := de.Info()
		if ierr != nil {
			continue
		}
		e := Entry{Name: de.Name(), IsDir: de.IsDir(), MTime: info.ModTime().Unix()}
		if de.Type()&fs.ModeSymlink != 0 {
			if st, serr := os.Stat(path + "/" + de.Name()); serr == nil && st.IsDir() {
				e.IsDir, e.Linked = true, true
			} else {
				continue
			}
		}
		if !e.IsDir {
			e.Size = info.Size()
			e.Alloc = e.Size
			if st, ok := info.Sys().(*syscall.Stat_t); ok {
				e.Alloc = st.Blocks * 512
			}
		}
		out = append(out, e)
	}
	return out, err
}

const onlineOnlySupported = false

func clusterSize(string) int64 { return 4096 }

func classifyErr(err error) string {
	if errors.Is(err, fs.ErrPermission) {
		return "denied"
	}
	return "error"
}

// SetBackupRead is only supported on Windows.
func SetBackupRead(bool) bool { return false }
