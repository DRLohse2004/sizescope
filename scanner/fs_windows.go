//go:build windows

package scanner

import (
	"errors"
	"strings"
	"sync/atomic"
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

const onlineOnlySupported = true

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
		if e1 == windows.ERROR_ACCESS_DENIED && backupRead.Load() {
			// protected folder (e.g. System Volume Information): list it with
			// backup semantics, which administrators may use regardless of ACLs
			return listDirBackup(path, cluster)
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
				e.Online = fd.Attr&(attrRecallOnAccess|attrOffline) != 0
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

// ---------------------------------------------------------------- backup semantics

var backupRead atomic.Bool

// SetBackupRead enables reading protected folders through SeBackupPrivilege.
// It only has an effect for elevated administrators; returns whether it is active.
func SetBackupRead(on bool) bool {
	if !on {
		backupRead.Store(false)
		return false
	}
	ok := enablePrivilege("SeBackupPrivilege")
	backupRead.Store(ok)
	return ok
}

func enablePrivilege(name string) bool {
	var tok windows.Token
	if windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_ADJUST_PRIVILEGES|windows.TOKEN_QUERY, &tok) != nil {
		return false
	}
	defer tok.Close()
	n, _ := windows.UTF16PtrFromString(name)
	var luid windows.LUID
	if windows.LookupPrivilegeValue(nil, n, &luid) != nil {
		return false
	}
	tp := windows.Tokenprivileges{PrivilegeCount: 1}
	tp.Privileges[0] = windows.LUIDAndAttributes{Luid: luid, Attributes: windows.SE_PRIVILEGE_ENABLED}
	// AdjustTokenPrivileges reports ERROR_NOT_ALL_ASSIGNED when the account lacks the privilege
	err := windows.AdjustTokenPrivileges(tok, false, &tp, 0, nil, nil)
	return err == nil
}

// FILE_FULL_DIR_INFO
type fullDirInfo struct {
	NextEntryOffset uint32
	FileIndex       uint32
	CreationTime    int64
	LastAccessTime  int64
	LastWriteTime   int64
	ChangeTime      int64
	EndOfFile       int64
	AllocationSize  int64
	FileAttributes  uint32
	FileNameLength  uint32
	EaSize          uint32 // reparse tag for reparse points
	FileName        [1]uint16
}

const fileFullDirectoryInfo = 14

func filetimeToUnix(ft int64) int64 { return ft/10000000 - 11644473600 }

func listDirBackup(path string, cluster int64) ([]Entry, error) {
	p16, err := windows.UTF16PtrFromString(extendedPath(path))
	if err != nil {
		return nil, err
	}
	h, err := windows.CreateFile(p16, windows.FILE_LIST_DIRECTORY,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil,
		windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		return nil, err
	}
	defer windows.CloseHandle(h)

	buf := make([]uint64, 8192) // 64 KiB, 8-byte aligned
	bb := (*byte)(unsafe.Pointer(&buf[0]))
	var out []Entry
	for {
		if err := windows.GetFileInformationByHandleEx(h, fileFullDirectoryInfo, bb, uint32(len(buf)*8)); err != nil {
			if err == windows.ERROR_NO_MORE_FILES {
				break
			}
			return out, err
		}
		off := uintptr(0)
		for {
			info := (*fullDirInfo)(unsafe.Add(unsafe.Pointer(bb), off))
			name := windows.UTF16ToString(unsafe.Slice(&info.FileName[0], info.FileNameLength/2))
			if name != "." && name != ".." {
				attr := info.FileAttributes
				e := Entry{Name: name, IsDir: attr&attrDirectory != 0, MTime: filetimeToUnix(info.LastWriteTime)}
				if attr&attrReparse != 0 && e.IsDir && !isCloudTag(info.EaSize) {
					e.Linked = true
				}
				if !e.IsDir {
					e.Online = attr&(attrRecallOnAccess|attrOffline) != 0
					e.Size = info.EndOfFile
					e.Alloc = info.AllocationSize
					if e.Online {
						e.Alloc = 0
					}
				}
				out = append(out, e)
			}
			if info.NextEntryOffset == 0 {
				break
			}
			off += uintptr(info.NextEntryOffset)
		}
	}
	_ = cluster
	return out, nil
}
