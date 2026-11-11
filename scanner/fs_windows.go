//go:build windows

package scanner

import (
	"errors"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	kernel32                  = windows.NewLazySystemDLL("kernel32.dll")
	procFindFirstFileExW      = kernel32.NewProc("FindFirstFileExW")
	procFindNextFileW         = kernel32.NewProc("FindNextFileW")
	procGetCompressedFileSize = kernel32.NewProc("GetCompressedFileSizeW")
	procGetDiskFreeSpaceW     = kernel32.NewProc("GetDiskFreeSpaceW")
)

// WIN32_FIND_DATAW
type findData struct {
	Attr      uint32
	Ctime     windows.Filetime
	Atime     windows.Filetime
	Mtime     windows.Filetime
	SizeHigh  uint32
	SizeLow   uint32
	Reserved0 uint32 // reparse tag when FILE_ATTRIBUTE_REPARSE_POINT is set
	Reserved1 uint32
	Name      [260]uint16
	Alt       [14]uint16
}

const (
	attrDirectory      = 0x10
	attrReparse        = 0x400
	attrSparse         = 0x200
	attrCompressed     = 0x800
	attrOffline        = 0x1000
	attrRecallOnOpen   = 0x40000
	attrRecallOnAccess = 0x400000

	findExInfoBasic         = 1
	findExLargeFetch        = 2
	findExOnDiskEntriesOnly = 4 // do not ask cloud providers (OneDrive etc.) to fetch missing entries
	invalidFileSize         = 0xFFFFFFFF
)

// extendedPath turns C:\x into \\?\C:\x and \\srv\share into \\?\UNC\srv\share
// so paths longer than MAX_PATH work.
func extendedPath(p string) string {
	if strings.HasPrefix(p, `\\?\`) {
		return p
	}
	if strings.HasPrefix(p, `\\`) {
		return `\\?\UNC\` + p[2:]
	}
	return `\\?\` + p
}

func isCloudTag(tag uint32) bool { return tag&0xFFFF0FFF == 0x9000001A }

func listDir(path string, cluster int64, cloudFetch bool) ([]Entry, error) {
	pattern := extendedPath(path)
	if !strings.HasSuffix(pattern, `\`) {
		pattern += `\`
	}
	pattern += "*"
	p16, err := windows.UTF16PtrFromString(pattern)
	if err != nil {
		return nil, err
	}
	var fd findData
	flags := uintptr(findExLargeFetch)
	if !cloudFetch {
		flags |= findExOnDiskEntriesOnly
	}
	h, _, e1 := procFindFirstFileExW.Call(uintptr(unsafe.Pointer(p16)), findExInfoBasic,
		uintptr(unsafe.Pointer(&fd)), 0, 0, flags)
	if windows.Handle(h) == windows.InvalidHandle && e1 == windows.ERROR_INVALID_PARAMETER && flags != findExLargeFetch {
		// older systems do not know the extra flag
		h, _, e1 = procFindFirstFileExW.Call(uintptr(unsafe.Pointer(p16)), findExInfoBasic,
			uintptr(unsafe.Pointer(&fd)), 0, 0, findExLargeFetch)
	}
	if windows.Handle(h) == windows.InvalidHandle {
		if e1 == windows.ERROR_FILE_NOT_FOUND {
			return nil, nil
		}
		return nil, e1
	}
	defer windows.FindClose(windows.Handle(h))

	var out []Entry
	for {
		name := windows.UTF16ToString(fd.Name[:])
		if name != "." && name != ".." {
			e := Entry{
				Name:  name,
				IsDir: fd.Attr&attrDirectory != 0,
				MTime: fd.Mtime.Nanoseconds() / 1e9,
			}
			if fd.Attr&attrReparse != 0 && e.IsDir && !isCloudTag(fd.Reserved0) {
				e.Linked = true // junction, symlink, mount point
			}
			if !e.IsDir {
				e.Size = int64(fd.SizeHigh)<<32 | int64(fd.SizeLow)
				e.Alloc = allocSize(path, name, fd.Attr, e.Size, cluster)
			}
			out = append(out, e)
		}
		r, _, e2 := procFindNextFileW.Call(h, uintptr(unsafe.Pointer(&fd)))
		if r == 0 {
			if e2 == windows.ERROR_NO_MORE_FILES {
				break
			}
			return out, e2
		}
	}
	return out, nil
}

// allocSize estimates the space a file occupies on disk without touching the
// file itself where possible:
//   - online-only cloud files (OneDrive "available online") occupy nothing
//   - NTFS-compressed or sparse files are asked for their real size
//   - everything else is rounded up to the cluster size
//
// Cloud files are never opened: every call would go through the cloud filter
// driver, which is slow with thousands of files.
func allocSize(dir, name string, attr uint32, size, cluster int64) int64 {
	if attr&(attrRecallOnAccess|attrRecallOnOpen|attrOffline) != 0 {
		return 0
	}
	if attr&attrReparse == 0 && attr&(attrCompressed|attrSparse) != 0 {
		p, err := windows.UTF16PtrFromString(extendedPath(joinPath(dir, name)))
		if err == nil {
			var high uint32
			low, _, e := procGetCompressedFileSize.Call(uintptr(unsafe.Pointer(p)), uintptr(unsafe.Pointer(&high)))
			if !(uint32(low) == invalidFileSize && e != windows.ERROR_SUCCESS) {
				return int64(high)<<32 | int64(uint32(low))
			}
		}
	}
	if cluster <= 0 || size == 0 {
		return size
	}
	return (size + cluster - 1) / cluster * cluster
}

func volumeRoot(path string) string {
	if strings.HasPrefix(path, `\\`) { // \\server\share\...
		parts := strings.SplitN(strings.TrimPrefix(path, `\\`), `\`, 3)
		if len(parts) >= 2 {
			return `\\` + parts[0] + `\` + parts[1] + `\`
		}
		return path
	}
	if len(path) >= 2 && path[1] == ':' {
		return path[:2] + `\`
	}
	return path
}

func clusterSize(path string) int64 {
	root, err := windows.UTF16PtrFromString(volumeRoot(path))
	if err != nil {
		return 4096
	}
	var spc, bps, free, total uint32
	r, _, _ := procGetDiskFreeSpaceW.Call(uintptr(unsafe.Pointer(root)),
		uintptr(unsafe.Pointer(&spc)), uintptr(unsafe.Pointer(&bps)),
		uintptr(unsafe.Pointer(&free)), uintptr(unsafe.Pointer(&total)))
	if r == 0 || spc == 0 || bps == 0 {
		return 4096
	}
	return int64(spc) * int64(bps)
}

func classifyErr(err error) string {
	var en syscall.Errno
	if errors.As(err, &en) && (en == windows.ERROR_ACCESS_DENIED || en == windows.ERROR_SHARING_VIOLATION) {
		return "denied"
	}
	return "error"
}
