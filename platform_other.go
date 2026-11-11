//go:build !windows

package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

func listDrives() []Drive {
	var out []Drive
	add := func(p, label string) {
		t, f := diskUsage(p)
		out = append(out, Drive{Path: p, Label: label, Kind: "fixed", Total: t, Free: f})
	}
	add("/", "")
	if h, err := os.UserHomeDir(); err == nil {
		add(h, filepath.Base(h))
	}
	return out
}

func diskUsage(path string) (int64, int64) {
	var st syscall.Statfs_t
	if syscall.Statfs(path, &st) != nil {
		return 0, 0
	}
	return int64(st.Blocks) * st.Bsize, int64(st.Bavail) * st.Bsize
}

func showInExplorer(path string, isDir bool) error {
	if !isDir {
		path = filepath.Dir(path)
	}
	return exec.Command("xdg-open", path).Start()
}

func openItem(path string) error { return exec.Command("xdg-open", path).Start() }

func moveToRecycleBin(path string) error {
	return errors.New("recycle bin is only supported on Windows")
}

func isAdmin() bool { return os.Geteuid() == 0 }

func restartAsAdmin([]string) error { return errors.New("not supported") }

const platformName = "other"
