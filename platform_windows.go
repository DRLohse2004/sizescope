//go:build windows

package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	shell32              = windows.NewLazySystemDLL("shell32.dll")
	procSHFileOperationW = shell32.NewProc("SHFileOperationW")
	procShellExecuteW    = shell32.NewProc("ShellExecuteW")
)

func listDrives() []Drive {
	var out []Drive
	// avoid "no disk in drive" popups for empty card readers / optical drives
	old := windows.SetErrorMode(windows.SEM_FAILCRITICALERRORS)
	defer windows.SetErrorMode(old)
	mask, err := windows.GetLogicalDrives()
	if err != nil {
		return out
	}
	for i := 0; i < 26; i++ {
		if mask&(1<<uint(i)) == 0 {
			continue
		}
		root := string(rune('A'+i)) + `:\`
		r16, _ := windows.UTF16PtrFromString(root)
		d := Drive{Path: root}
		switch windows.GetDriveType(r16) {
		case windows.DRIVE_REMOVABLE:
			d.Kind = "removable"
		case windows.DRIVE_FIXED:
			d.Kind = "fixed"
		case windows.DRIVE_REMOTE:
			d.Kind = "network"
		case windows.DRIVE_CDROM:
			d.Kind = "cdrom"
		case windows.DRIVE_RAMDISK:
			d.Kind = "ram"
		default:
			d.Kind = "other"
		}
		d.Total, d.Free = diskUsage(root)
		label := make([]uint16, 261)
		fs := make([]uint16, 261)
		if windows.GetVolumeInformation(r16, &label[0], uint32(len(label)), nil, nil, nil, &fs[0], uint32(len(fs))) == nil {
			d.Label = windows.UTF16ToString(label)
			d.FS = windows.UTF16ToString(fs)
		}
		out = append(out, d)
	}
	return out
}

func diskUsage(path string) (total, free int64) {
	p16, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, 0
	}
	var avail, tot, totFree uint64
	if windows.GetDiskFreeSpaceEx(p16, &avail, &tot, &totFree) != nil {
		return 0, 0
	}
	return int64(tot), int64(totFree)
}

func showInExplorer(path string, isDir bool) error {
	cmd := exec.Command("explorer.exe")
	if isDir {
		cmd.SysProcAttr = &syscall.SysProcAttr{CmdLine: `explorer.exe "` + path + `"`}
	} else {
		cmd.SysProcAttr = &syscall.SysProcAttr{CmdLine: `explorer.exe /select,"` + path + `"`}
	}
	return cmd.Start()
}

func shellExecute(verb, file, args string) error {
	v, _ := windows.UTF16PtrFromString(verb)
	f, _ := windows.UTF16PtrFromString(file)
	var a *uint16
	if args != "" {
		a, _ = windows.UTF16PtrFromString(args)
	}
	r, _, _ := procShellExecuteW.Call(0, uintptr(unsafe.Pointer(v)), uintptr(unsafe.Pointer(f)),
		uintptr(unsafe.Pointer(a)), 0, windows.SW_SHOWNORMAL)
	if r <= 32 {
		return fmt.Errorf("ShellExecute failed (%d)", r)
	}
	return nil
}

func openItem(path string) error { return shellExecute("open", path, "") }

type shFileOpStruct struct {
	Hwnd          uintptr
	Func          uint32
	From          *uint16
	To            *uint16
	Flags         uint16
	AnyAborted    int32
	NameMappings  uintptr
	ProgressTitle *uint16
}

const (
	foDelete        = 3
	fofSilent       = 0x4
	fofNoConfirm    = 0x10
	fofAllowUndo    = 0x40
	fofWantNukeWarn = 0x4000
)

// moveToRecycleBin moves a file or folder to the recycle bin. Windows warns
// itself if an item is too large for the bin and would be deleted permanently.
func moveToRecycleBin(path string) error {
	from, err := windows.UTF16FromString(path)
	if err != nil {
		return err
	}
	from = append(from, 0) // double NUL terminated
	op := shFileOpStruct{Func: foDelete, From: &from[0], Flags: fofAllowUndo | fofNoConfirm | fofSilent | fofWantNukeWarn}
	r, _, _ := procSHFileOperationW.Call(uintptr(unsafe.Pointer(&op)))
	if r != 0 {
		return fmt.Errorf("delete failed (code 0x%x)", r)
	}
	if op.AnyAborted != 0 {
		return fmt.Errorf("aborted")
	}
	if _, err := os.Lstat(path); err == nil {
		return fmt.Errorf("item still exists")
	}
	return nil
}

func isAdmin() bool {
	return windows.GetCurrentProcessToken().IsElevated()
}

func restartAsAdmin(args []string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	quoted := make([]string, len(args))
	for i, a := range args {
		quoted[i] = syscall.EscapeArg(a)
	}
	return shellExecute("runas", exe, strings.Join(quoted, " "))
}

const platformName = "windows"
