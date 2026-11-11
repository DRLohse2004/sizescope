//go:build !windows

package cleanup

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"syscall"
)

var errUnsupported = errors.New("only supported on Windows")

func stopService(string) (bool, error) { return false, errUnsupported }
func startService(string) error        { return errUnsupported }

func recycleBinInfo() (int64, int64, error) { return 0, 0, errUnsupported }
func emptyRecycleBin() error                { return errUnsupported }

func runCommand(ctx context.Context, command string) (string, error) {
	out, err := exec.CommandContext(ctx, "sh", "-c", command).CombinedOutput()
	return string(out), err
}

func runScript(ctx context.Context, path string, dryRun bool) (string, error) {
	cmd := exec.CommandContext(ctx, "pwsh", "-NoProfile", "-NonInteractive", "-File", path)
	cmd.Env = os.Environ()
	if dryRun {
		cmd.Env = append(cmd.Env, "SIZESCOPE_DRYRUN=1")
	}
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func freeSpace(path string) int64 {
	var st syscall.Statfs_t
	if syscall.Statfs(path, &st) != nil {
		return 0
	}
	return int64(st.Bavail) * st.Bsize
}
