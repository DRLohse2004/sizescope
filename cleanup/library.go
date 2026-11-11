package cleanup

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func bp(b bool) *bool { return &b }

// Builtin returns the cleanups shipped with SizeScope.
func Builtin() []*Definition {
	defs := []*Definition{
		{
			ID: "user-temp", Default: true,
			Name:        Text{"de": "Temporäre Dateien (eigener Benutzer)", "en": "Temporary files (current user)"},
			Description: Text{"de": "%TEMP%, nur Dateien älter als 1 Tag.", "en": "%TEMP%, only files older than 1 day."},
			Targets:     []Target{{Path: `%TEMP%`, MinAgeDays: 1}},
		},
		{
			ID: "all-users-temp", Admin: true,
			Name:        Text{"de": "Temporäre Dateien (alle Benutzer)", "en": "Temporary files (all users)"},
			Description: Text{"de": "Temp-Ordner aller Benutzerprofile, älter als 1 Tag – nützlich auf Terminalservern.", "en": "Temp folders of all user profiles, older than 1 day – useful on terminal servers."},
			Targets:     []Target{{Path: `%SystemDrive%\Users\*\AppData\Local\Temp`, MinAgeDays: 1}},
		},
		{
			ID: "windows-temp", Admin: true, Default: true,
			Name:        Text{"de": "Windows-Temp", "en": "Windows temp"},
			Description: Text{"de": "%SystemRoot%\\Temp, älter als 1 Tag.", "en": "%SystemRoot%\\Temp, older than 1 day."},
			Targets:     []Target{{Path: `%SystemRoot%\Temp`, MinAgeDays: 1}},
		},
		{
			ID: "wu-downloads", Admin: true,
			Name:         Text{"de": "Windows-Update-Downloads", "en": "Windows Update downloads"},
			Description:  Text{"de": "Heruntergeladene Update-Pakete. Die Dienste wuauserv und BITS werden dafür kurz gestoppt.", "en": "Downloaded update packages. The wuauserv and BITS services are stopped briefly."},
			StopServices: []string{"wuauserv", "bits"},
			Targets:      []Target{{Path: `%SystemRoot%\SoftwareDistribution\Download`}},
		},
		{
			ID: "delivery-optimization", Admin: true,
			Name:         Text{"de": "Übermittlungsoptimierung", "en": "Delivery Optimization cache"},
			Description:  Text{"de": "Zwischenspeicher für Updates, die mit anderen PCs geteilt werden. Der Dienst DoSvc wird kurz gestoppt.", "en": "Cache for updates shared with other PCs. The DoSvc service is stopped briefly."},
			StopServices: []string{"dosvc"},
			Targets:      []Target{{Path: `%SystemRoot%\ServiceProfiles\NetworkService\AppData\Local\Microsoft\Windows\DeliveryOptimization\Cache`}},
		},
		{
			ID: "error-reports", Admin: true,
			Name:        Text{"de": "Fehlerberichte und Speicherabbilder", "en": "Error reports and memory dumps"},
			Description: Text{"de": "Windows-Fehlerberichte, Minidumps, MEMORY.DMP und Absturzabbilder von Programmen.", "en": "Windows error reports, minidumps, MEMORY.DMP and application crash dumps."},
			Targets: []Target{
				{Path: `%ProgramData%\Microsoft\Windows\WER\ReportArchive`},
				{Path: `%ProgramData%\Microsoft\Windows\WER\ReportQueue`},
				{Path: `%ProgramData%\Microsoft\Windows\WER\Temp`},
				{Path: `%SystemRoot%\Minidump`},
				{Path: `%SystemRoot%\LiveKernelReports`, Include: []string{"*.dmp"}},
				{Path: `%SystemRoot%`, Include: []string{"MEMORY.DMP"}, Recurse: bp(false)},
				{Path: `%SystemDrive%\Users\*\AppData\Local\CrashDumps`},
			},
		},
		{
			ID:          "browser-cache",
			Name:        Text{"de": "Browser-Caches (eigener Benutzer)", "en": "Browser caches (current user)"},
			Description: Text{"de": "Cache von Edge, Chrome und Firefox. Bei geöffnetem Browser werden belegte Dateien übersprungen.", "en": "Edge, Chrome and Firefox cache. Files in use are skipped while the browser is open."},
			Targets: []Target{
				{Path: `%LOCALAPPDATA%\Microsoft\Edge\User Data\*\Cache`},
				{Path: `%LOCALAPPDATA%\Microsoft\Edge\User Data\*\Code Cache`},
				{Path: `%LOCALAPPDATA%\Microsoft\Edge\User Data\*\GPUCache`},
				{Path: `%LOCALAPPDATA%\Google\Chrome\User Data\*\Cache`},
				{Path: `%LOCALAPPDATA%\Google\Chrome\User Data\*\Code Cache`},
				{Path: `%LOCALAPPDATA%\Google\Chrome\User Data\*\GPUCache`},
				{Path: `%LOCALAPPDATA%\Mozilla\Firefox\Profiles\*\cache2`},
			},
		},
		{
			ID:          "thumbnail-cache",
			Name:        Text{"de": "Miniaturansichten-Cache", "en": "Thumbnail cache"},
			Description: Text{"de": "Vorschaubilder des Explorers; werden bei Bedarf neu erzeugt. Belegte Dateien werden übersprungen.", "en": "Explorer thumbnails; recreated when needed. Files in use are skipped."},
			Targets:     []Target{{Path: `%LOCALAPPDATA%\Microsoft\Windows\Explorer`, Include: []string{"thumbcache_*.db", "iconcache_*.db"}, Recurse: bp(false)}},
		},
		{
			ID:          "recycle-bin",
			Name:        Text{"de": "Papierkorb leeren", "en": "Empty recycle bin"},
			Description: Text{"de": "Papierkorb aller Laufwerke des eigenen Benutzers.", "en": "Recycle bin of all drives for the current user."},
			RecycleBin:  true,
		},
		{
			ID: "iis-logs", Admin: true,
			Name:        Text{"de": "IIS-Protokolle älter als 30 Tage", "en": "IIS logs older than 30 days"},
			Description: Text{"de": "%SystemDrive%\\inetpub\\logs\\LogFiles – für Server mit IIS.", "en": "%SystemDrive%\\inetpub\\logs\\LogFiles – for servers running IIS."},
			Targets:     []Target{{Path: `%SystemDrive%\inetpub\logs\LogFiles`, Include: []string{"*.log"}, MinAgeDays: 30, RemoveEmptyDirs: bp(false)}},
		},
		{
			ID: "windows-logs", Admin: true,
			Name:        Text{"de": "Windows-Setup-Protokolle älter als 30 Tage", "en": "Windows setup logs older than 30 days"},
			Description: Text{"de": "CBS- und DISM-Protokolle unter %SystemRoot%\\Logs.", "en": "CBS and DISM logs below %SystemRoot%\\Logs."},
			Targets: []Target{
				{Path: `%SystemRoot%\Logs\CBS`, Include: []string{"*.log", "*.cab"}, MinAgeDays: 30, RemoveEmptyDirs: bp(false)},
				{Path: `%SystemRoot%\Logs\DISM`, Include: []string{"*.log"}, MinAgeDays: 30, RemoveEmptyDirs: bp(false)},
			},
		},
		{
			ID: "component-store", Admin: true, Slow: true,
			Name:        Text{"de": "Komponentenspeicher (WinSxS) aufräumen", "en": "Clean up component store (WinSxS)"},
			Description: Text{"de": "DISM /StartComponentCleanup entfernt alte Update-Komponenten. Kann mehrere Minuten dauern; Platzgewinn wird über den freien Speicher gemessen.", "en": "DISM /StartComponentCleanup removes superseded update components. May take several minutes; space is measured via free disk space."},
			Commands:    []string{`Dism.exe /Online /Cleanup-Image /StartComponentCleanup`},
		},
	}
	for _, d := range defs {
		d.Builtin = true
	}
	return defs
}

// LoadDir reads imported definitions (*.json and *.ps1) from dir.
func LoadDir(dir string) ([]*Definition, []error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, nil
	}
	var defs []*Definition
	var errs []error
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		p := filepath.Join(dir, e.Name())
		ds, err := LoadFile(p)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", e.Name(), err))
			continue
		}
		defs = append(defs, ds...)
	}
	sort.SliceStable(defs, func(i, j int) bool { return defs[i].Name.Get("") < defs[j].Name.Get("") })
	return defs, errs
}

// LoadFile parses one definition file.
func LoadFile(p string) ([]*Definition, error) {
	switch strings.ToLower(filepath.Ext(p)) {
	case ".json":
		b, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		var list []*Definition
		if err := json.Unmarshal(b, &list); err != nil {
			var one Definition
			if err2 := json.Unmarshal(b, &one); err2 != nil {
				return nil, err2
			}
			list = []*Definition{&one}
		}
		for i, d := range list {
			if len(d.Name) == 0 {
				return nil, errors.New("definition without name")
			}
			if len(d.Targets) == 0 && len(d.Commands) == 0 && !d.RecycleBin {
				return nil, fmt.Errorf("%q has nothing to do", d.Name.Get(""))
			}
			d.File = p
			d.ID = fmt.Sprintf("file:%s#%d", filepath.Base(p), i)
		}
		return list, nil
	case ".ps1":
		return []*Definition{parseScriptHeader(p)}, nil
	}
	return nil, nil // other files are ignored
}

// parseScriptHeader reads optional metadata from leading comment lines:
//
//	# Name: My cleanup          (or Name-de: / Name-en:)
//	# Description: What it does (or Description-de: / -en:)
//	# Admin: true
//	# Slow: true
//	# Preview: true             (script honours SIZESCOPE_DRYRUN=1)
func parseScriptHeader(p string) *Definition {
	d := &Definition{ID: "file:" + filepath.Base(p), File: p, Script: p, Name: Text{}, Description: Text{}}
	f, err := os.Open(p)
	if err == nil {
		defer f.Close()
		sc := bufio.NewScanner(f)
		for i := 0; sc.Scan() && i < 40; i++ {
			line := strings.TrimSpace(sc.Text())
			if line == "" {
				continue
			}
			if !strings.HasPrefix(line, "#") {
				break
			}
			k, v, ok := strings.Cut(strings.TrimSpace(strings.TrimPrefix(line, "#")), ":")
			if !ok {
				continue
			}
			k, v = strings.ToLower(strings.TrimSpace(k)), strings.TrimSpace(v)
			key, lang, _ := strings.Cut(k, "-")
			switch key {
			case "name":
				d.Name[lang] = v
			case "description":
				d.Description[lang] = v
			case "admin":
				d.Admin = strings.EqualFold(v, "true") || v == "1"
			case "slow":
				d.Slow = strings.EqualFold(v, "true") || v == "1"
			case "preview":
				d.Preview = strings.EqualFold(v, "true") || v == "1"
			}
		}
	}
	if len(d.Name) == 0 {
		d.Name[""] = strings.TrimSuffix(filepath.Base(p), filepath.Ext(p))
	}
	return d
}

// Import validates src and copies it into dir; returns the new file path.
func Import(src, dir string) (string, error) {
	ext := strings.ToLower(filepath.Ext(src))
	if ext != ".json" && ext != ".ps1" {
		return "", errors.New("only .json and .ps1 files can be imported")
	}
	if _, err := LoadFile(src); err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	base := strings.TrimSuffix(filepath.Base(src), filepath.Ext(src))
	dst := filepath.Join(dir, base+ext)
	for i := 2; ; i++ {
		if _, err := os.Stat(dst); errors.Is(err, os.ErrNotExist) {
			break
		}
		dst = filepath.Join(dir, fmt.Sprintf("%s (%d)%s", base, i, ext))
	}
	in, err := os.Open(src)
	if err != nil {
		return "", err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return "", err
	}
	return dst, out.Close()
}
