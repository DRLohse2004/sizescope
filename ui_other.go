//go:build !windows

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gioui.org/font"
	"gioui.org/io/event"
)

func windowHandle(event.Event) uintptr { return 0 }

func systemFontFiles() []fontFile {
	var out []fontFile
	for _, f := range []struct {
		name string
		w    font.Weight
	}{
		{"/usr/share/fonts/truetype/noto/NotoSans-Regular.ttf", font.Normal},
		{"/usr/share/fonts/truetype/noto/NotoSans-SemiBold.ttf", font.SemiBold},
		{"/usr/share/fonts/truetype/noto/NotoSans-Bold.ttf", font.Bold},
	} {
		if b, err := os.ReadFile(f.name); err == nil {
			out = append(out, fontFile{b, f.w})
		}
	}
	return out
}

func systemLanguage() string {
	if strings.HasPrefix(os.Getenv("LANG"), "de") {
		return "de"
	}
	return "en"
}

func systemDarkMode() bool { return os.Getenv("SIZESCOPE_DARK") == "1" }

func setTitleBarDark(uintptr, bool) {}

func confirm(_ uintptr, _, _ string) bool { return true }

func pickFolderDialog(uintptr) string {
	out, err := exec.Command("zenity", "--file-selection", "--directory").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func saveFileDialog(_ uintptr, def string) string { return filepath.Join(os.TempDir(), def) }
