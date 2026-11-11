//go:build windows

package main

import (
	"os"
	"path/filepath"
	"runtime"
	"unsafe"

	"gioui.org/app"
	"gioui.org/font"
	"gioui.org/io/event"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

var (
	user32                       = windows.NewLazySystemDLL("user32.dll")
	procMessageBoxW              = user32.NewProc("MessageBoxW")
	dwmapi                       = windows.NewLazySystemDLL("dwmapi.dll")
	procDwmSetWindowAttr         = dwmapi.NewProc("DwmSetWindowAttribute")
	comdlg32                     = windows.NewLazySystemDLL("comdlg32.dll")
	procGetSaveFileNameW         = comdlg32.NewProc("GetSaveFileNameW")
	ole32                        = windows.NewLazySystemDLL("ole32.dll")
	procCoInitializeEx           = ole32.NewProc("CoInitializeEx")
	procCoTaskMemFree            = ole32.NewProc("CoTaskMemFree")
	procSHBrowseForFolderW       = shell32.NewProc("SHBrowseForFolderW")
	procSHGetPathFromIDListW     = shell32.NewProc("SHGetPathFromIDListW")
	procGetUserDefaultUILanguage = windows.NewLazySystemDLL("kernel32.dll").NewProc("GetUserDefaultUILanguage")
)

func windowHandle(e event.Event) uintptr {
	if v, ok := e.(app.Win32ViewEvent); ok {
		return v.HWND
	}
	return 0
}

func systemFontFiles() []fontFile {
	dir := filepath.Join(os.Getenv("WINDIR"), "Fonts")
	if os.Getenv("WINDIR") == "" {
		dir = `C:\Windows\Fonts`
	}
	var out []fontFile
	for _, f := range []struct {
		name string
		w    font.Weight
	}{{"segoeui.ttf", font.Normal}, {"seguisb.ttf", font.SemiBold}, {"segoeuib.ttf", font.Bold}} {
		if b, err := os.ReadFile(filepath.Join(dir, f.name)); err == nil {
			out = append(out, fontFile{b, f.w})
		}
	}
	return out
}

func systemLanguage() string {
	r, _, _ := procGetUserDefaultUILanguage.Call()
	if uint16(r)&0x3ff == 0x07 {
		return "de"
	}
	return "en"
}

func systemDarkMode() bool {
	k, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Themes\Personalize`, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()
	v, _, err := k.GetIntegerValue("AppsUseLightTheme")
	return err == nil && v == 0
}

func setTitleBarDark(hwnd uintptr, dark bool) {
	var v int32
	if dark {
		v = 1
	}
	// DWMWA_USE_IMMERSIVE_DARK_MODE = 20 (19 on early Windows 10 builds)
	for _, attr := range []uintptr{20, 19} {
		r, _, _ := procDwmSetWindowAttr.Call(hwnd, attr, uintptr(unsafe.Pointer(&v)), 4)
		if r == 0 {
			return
		}
	}
}

const (
	mbYesNo       = 0x4
	mbIconWarning = 0x30
	mbDefButton2  = 0x100
	idYes         = 6
)

func confirm(hwnd uintptr, title, msg string) bool {
	t, _ := windows.UTF16PtrFromString(title)
	m, _ := windows.UTF16PtrFromString(msg)
	r, _, _ := procMessageBoxW.Call(hwnd, uintptr(unsafe.Pointer(m)), uintptr(unsafe.Pointer(t)), mbYesNo|mbIconWarning|mbDefButton2)
	return r == idYes
}

type browseInfo struct {
	Owner       uintptr
	Root        uintptr
	DisplayName *uint16
	Title       *uint16
	Flags       uint32
	Callback    uintptr
	LParam      uintptr
	Image       int32
}

func pickFolderDialog(hwnd uintptr) string {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	procCoInitializeEx.Call(0, 0x2) // COINIT_APARTMENTTHREADED
	name := make([]uint16, windows.MAX_PATH)
	bi := browseInfo{Owner: hwnd, DisplayName: &name[0], Flags: 0x1 | 0x10 | 0x40} // only dirs, edit box, new style
	pidl, _, _ := procSHBrowseForFolderW.Call(uintptr(unsafe.Pointer(&bi)))
	if pidl == 0 {
		return ""
	}
	defer procCoTaskMemFree.Call(pidl)
	buf := make([]uint16, 32768)
	if r, _, _ := procSHGetPathFromIDListW.Call(pidl, uintptr(unsafe.Pointer(&buf[0]))); r == 0 {
		return ""
	}
	return windows.UTF16ToString(buf)
}

type openFileName struct {
	StructSize    uint32
	Owner         uintptr
	Instance      uintptr
	Filter        *uint16
	CustomFilter  *uint16
	MaxCustFilter uint32
	FilterIndex   uint32
	File          *uint16
	MaxFile       uint32
	FileTitle     *uint16
	MaxFileTitle  uint32
	InitialDir    *uint16
	Title         *uint16
	Flags         uint32
	FileOffset    uint16
	FileExtension uint16
	DefExt        *uint16
	CustData      uintptr
	FnHook        uintptr
	TemplateName  *uint16
	Reserved      uintptr
	Reserved2     uint32
	FlagsEx       uint32
}

func saveFileDialog(hwnd uintptr, defName string) string {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	buf := make([]uint16, 32768)
	copy(buf, windows.StringToUTF16(defName))
	filter := windows.StringToUTF16("CSV (*.csv)\x00*.csv\x00\x00")
	ext, _ := windows.UTF16PtrFromString("csv")
	ofn := openFileName{Owner: hwnd, Filter: &filter[0], File: &buf[0], MaxFile: uint32(len(buf)), DefExt: ext,
		Flags: 0x2 | 0x800 | 0x80000} // overwrite prompt, path must exist, explorer
	ofn.StructSize = uint32(unsafe.Sizeof(ofn))
	if r, _, _ := procGetSaveFileNameW.Call(uintptr(unsafe.Pointer(&ofn))); r == 0 {
		return ""
	}
	return windows.UTF16ToString(buf)
}
