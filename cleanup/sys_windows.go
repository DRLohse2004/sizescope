//go:build windows

package cleanup

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

var (
	shell32                = windows.NewLazySystemDLL("shell32.dll")
	procSHQueryRecycleBinW = shell32.NewProc("SHQueryRecycleBinW")
	procSHEmptyRecycleBinW = shell32.NewProc("SHEmptyRecycleBinW")
)

const createNoWindow = 0x08000000

func stopService(name string) (wasRunning bool, err error) {
	m, err := mgr.Connect()
	if err != nil {
		return false, err
	}
	defer m.Disconnect()
	s, err := m.OpenService(name)
	if err != nil {
		return false, err
	}
	defer s.Close()
	st, err := s.Query()
	if err != nil {
		return false, err
	}
	if st.State == svc.Stopped {
		return false, nil
	}
	if _, err := s.Control(svc.Stop); err != nil {
		return true, err
	}
	for i := 0; i < 60; i++ {
		if st, err = s.Query(); err == nil && st.State == svc.Stopped {
			return true, nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return true, errors.New("timeout while stopping")
}

func startService(name string) error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()
	s, err := m.OpenService(name)
	if err != nil {
		return err
	}
	defer s.Close()
	return s.Start()
}

type shQueryRBInfo struct {
	Size     uint32
	Bytes    int64
	NumItems int64
}

func recycleBinInfo() (bytes, files int64, err error) {
	info := shQueryRBInfo{Size: uint32(unsafe.Sizeof(shQueryRBInfo{}))}
	r, _, _ := procSHQueryRecycleBinW.Call(0, uintptr(unsafe.Pointer(&info)))
	if r != 0 {
		return 0, 0, fmt.Errorf("SHQueryRecycleBin failed (0x%x)", r)
	}
	return info.Bytes, info.NumItems, nil
}

func emptyRecycleBin() error {
	r, _, _ := procSHEmptyRecycleBinW.Call(0, 0, 0x1|0x2|0x4) // no confirmation, no progress, no sound
	if r != 0 {
		return fmt.Errorf("SHEmptyRecycleBin failed (0x%x)", r)
	}
	return nil
}

func hidden(cmd *exec.Cmd, cmdline string) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow, CmdLine: cmdline}
}

func runCommand(ctx context.Context, command string) (string, error) {
	cmd := exec.CommandContext(ctx, "cmd.exe")
	hidden(cmd, `cmd.exe /d /c `+command)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func runScript(ctx context.Context, path string, dryRun bool) (string, error) {
	cmd := exec.CommandContext(ctx, "powershell.exe")
	hidden(cmd, `powershell.exe -NoProfile -NonInteractive -ExecutionPolicy Bypass -File "`+path+`"`)
	cmd.Env = os.Environ()
	if dryRun {
		cmd.Env = append(cmd.Env, "SIZESCOPE_DRYRUN=1")
	}
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func freeSpace(path string) int64 {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0
	}
	var avail, total, free uint64
	if windows.GetDiskFreeSpaceEx(p, &avail, &total, &free) != nil {
		return 0
	}
	return int64(free)
}
