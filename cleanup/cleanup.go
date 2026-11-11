// Package cleanup runs cleanup definitions: declarative rules (folders,
// name patterns, minimum age), shell commands and PowerShell scripts.
// Rules support a preview (dry run) and report exactly what was removed where.
package cleanup

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Text is a translatable string: either "text" or {"de": "...", "en": "..."} in JSON.
type Text map[string]string

func (t *Text) UnmarshalJSON(b []byte) error {
	var s string
	if json.Unmarshal(b, &s) == nil {
		*t = Text{"": s}
		return nil
	}
	m := map[string]string{}
	if err := json.Unmarshal(b, &m); err != nil {
		return err
	}
	*t = m
	return nil
}

// Get returns the text for lang, falling back to English and then any value.
func (t Text) Get(lang string) string {
	for _, k := range []string{lang, "en", ""} {
		if v, ok := t[k]; ok && v != "" {
			return v
		}
	}
	for _, v := range t {
		return v
	}
	return ""
}

// Target is a folder to clean. Path may contain %VARIABLES% and * wildcards
// in any segment (e.g. C:\Users\*\AppData\Local\Temp).
type Target struct {
	Path            string   `json:"path"`
	Include         []string `json:"include,omitempty"` // file name patterns, default *
	Exclude         []string `json:"exclude,omitempty"`
	MinAgeDays      float64  `json:"minAgeDays,omitempty"`
	Recurse         *bool    `json:"recurse,omitempty"`         // default true
	RemoveEmptyDirs *bool    `json:"removeEmptyDirs,omitempty"` // default true
}

// Definition is one selectable cleanup.
type Definition struct {
	ID           string   `json:"id"`
	Name         Text     `json:"name"`
	Description  Text     `json:"description,omitempty"`
	Admin        bool     `json:"admin,omitempty"`   // needs administrator rights
	Default      bool     `json:"default,omitempty"` // preselected
	Slow         bool     `json:"slow,omitempty"`    // may take several minutes
	StopServices []string `json:"stopServices,omitempty"`
	Targets      []Target `json:"targets,omitempty"`
	Commands     []string `json:"commands,omitempty"`
	RecycleBin   bool     `json:"recycleBin,omitempty"`

	Script  string `json:"-"` // path of a PowerShell script
	Preview bool   `json:"-"` // script supports SIZESCOPE_DRYRUN=1
	Builtin bool   `json:"-"`
	File    string `json:"-"` // file the definition was imported from
}

// Location is what happened at one place.
type Location struct {
	Path         string
	Files        int64
	Bytes        int64
	Skipped      int64 // files that could not be deleted (in use, access denied)
	SkippedBytes int64
	Note         string
}

// Result of one definition.
type Result struct {
	Def       *Definition
	Locations []Location
	Files     int64
	Bytes     int64
	Skipped   int64
	Err       string
	Duration  time.Duration
	DryRun    bool
	Estimated bool // bytes measured as free-space difference
}

// Progress is reported while running.
type Progress struct {
	Def   string
	Path  string
	Files int64
	Bytes int64
}

// Run executes the definitions one after another.
func Run(ctx context.Context, defs []*Definition, dryRun bool, progress func(Progress)) []Result {
	var out []Result
	for _, d := range defs {
		if ctx.Err() != nil {
			break
		}
		out = append(out, runOne(ctx, d, dryRun, progress))
	}
	return out
}

func runOne(ctx context.Context, d *Definition, dryRun bool, progress func(Progress)) Result {
	start := time.Now()
	res := Result{Def: d, DryRun: dryRun}
	report := func(path string, files, bytes int64) {
		if progress != nil {
			progress(Progress{Def: d.ID, Path: path, Files: res.Files + files, Bytes: res.Bytes + bytes})
		}
	}

	var restart []string
	if !dryRun {
		for _, s := range d.StopServices {
			running, err := stopService(s)
			if err != nil {
				res.Locations = append(res.Locations, Location{Path: "service " + s, Note: err.Error()})
				continue
			}
			if running {
				restart = append(restart, s)
			}
		}
	}

	for _, t := range d.Targets {
		for _, loc := range cleanTarget(ctx, t, dryRun, report) {
			res.add(loc)
		}
	}
	if d.RecycleBin {
		res.add(recycleBin(dryRun))
	}
	for _, c := range d.Commands {
		res.add(runMeasured(ctx, c, "", dryRun))
		res.Estimated = true
	}
	if d.Script != "" {
		locs, est := runScriptMeasured(ctx, d.Script, dryRun, d.Preview)
		for _, l := range locs {
			res.add(l)
		}
		res.Estimated = est
	}

	for _, s := range restart {
		if err := startService(s); err != nil {
			res.Err = "service " + s + ": " + err.Error()
		}
	}
	if ctx.Err() != nil {
		res.Err = "canceled"
	}
	res.Duration = time.Since(start)
	return res
}

func (r *Result) add(l Location) {
	r.Locations = append(r.Locations, l)
	r.Files += l.Files
	r.Bytes += l.Bytes
	r.Skipped += l.Skipped
}

// ---------------------------------------------------------------- targets

var envRe = regexp.MustCompile(`%([A-Za-z0-9_()]+)%`)

// ExpandPath replaces %VAR% with environment values; ok is false when a
// variable is unset (the target is skipped then, never cleaned at the root).
func ExpandPath(p string) (string, bool) {
	ok := true
	out := envRe.ReplaceAllStringFunc(p, func(m string) string {
		v := os.Getenv(m[1 : len(m)-1])
		if v == "" {
			ok = false
		}
		return v
	})
	return filepath.Clean(out), ok
}

// safeBase refuses to clean volume roots and top-level system folders.
func safeBase(p string) bool {
	if !filepath.IsAbs(p) {
		return false
	}
	vol := filepath.VolumeName(p)
	rest := strings.Trim(strings.TrimPrefix(p, vol), `\/`)
	if rest == "" {
		return false
	}
	parts := strings.FieldsFunc(rest, func(r rune) bool { return r == '\\' || r == '/' })
	if len(parts) < 2 {
		return false // e.g. C:\Windows, C:\Users, /home
	}
	low := strings.ToLower(rest)
	for _, bad := range []string{"windows/system32", `windows\system32`, "windows/winsxs", `windows\winsxs`} {
		if low == bad {
			return false
		}
	}
	return true
}

func boolOr(b *bool, def bool) bool {
	if b == nil {
		return def
	}
	return *b
}

func matchAny(patterns []string, name string) bool {
	ln := strings.ToLower(name)
	for _, p := range patterns {
		if ok, _ := filepath.Match(strings.ToLower(p), ln); ok {
			return true
		}
	}
	return false
}

func cleanTarget(ctx context.Context, t Target, dryRun bool, report func(string, int64, int64)) []Location {
	pattern, ok := ExpandPath(t.Path)
	if !ok {
		return nil
	}
	bases, _ := filepath.Glob(pattern)
	if !strings.ContainsAny(pattern, "*?[") {
		bases = []string{pattern}
	}
	sort.Strings(bases)
	include := t.Include
	if len(include) == 0 {
		include = []string{"*"}
	}
	recurse := boolOr(t.Recurse, true)
	removeDirs := boolOr(t.RemoveEmptyDirs, true)
	cutoff := time.Now().Add(-time.Duration(t.MinAgeDays * 24 * float64(time.Hour)))

	var out []Location
	for _, base := range bases {
		st, err := os.Stat(base)
		if err != nil || !st.IsDir() {
			continue
		}
		// a non-recursive target with exact file names may sit higher up (e.g. %SystemRoot%\MEMORY.DMP)
		exact := !recurse
		for _, p := range include {
			if strings.ContainsAny(p, "*?[") {
				exact = false
			}
		}
		if !safeBase(base) && !exact {
			out = append(out, Location{Path: base, Note: "refused: too close to the drive root"})
			continue
		}
		loc := Location{Path: base}
		var dirs []string
		n := 0
		_ = filepath.WalkDir(base, func(p string, d fs.DirEntry, err error) error {
			if ctx.Err() != nil {
				return filepath.SkipAll
			}
			if err != nil {
				if p != base {
					loc.Skipped++
				}
				return nil
			}
			if p == base {
				return nil
			}
			// never follow or delete links and junctions
			if d.Type()&(fs.ModeSymlink|fs.ModeIrregular) != 0 {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if d.IsDir() {
				if !recurse {
					return filepath.SkipDir
				}
				dirs = append(dirs, p)
				return nil
			}
			name := d.Name()
			if !matchAny(include, name) || matchAny(t.Exclude, name) {
				return nil
			}
			info, err := d.Info()
			if err != nil {
				loc.Skipped++
				return nil
			}
			if t.MinAgeDays > 0 && info.ModTime().After(cutoff) {
				return nil
			}
			size := info.Size()
			if !dryRun {
				if err := removeFile(p); err != nil {
					loc.Skipped++
					loc.SkippedBytes += size
					return nil
				}
			}
			loc.Files++
			loc.Bytes += size
			if n++; n%200 == 0 {
				report(base, loc.Files, loc.Bytes)
			}
			return nil
		})
		if !dryRun && removeDirs {
			// deepest first; folders that still contain something stay
			sort.Slice(dirs, func(i, j int) bool { return len(dirs[i]) > len(dirs[j]) })
			for _, d := range dirs {
				_ = os.Remove(d)
			}
		}
		report(base, loc.Files, loc.Bytes)
		out = append(out, loc)
	}
	return out
}

func removeFile(p string) error {
	err := os.Remove(p)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		// read-only files: clear the attribute and retry
		if os.Chmod(p, 0o666) == nil {
			err = os.Remove(p)
		}
	}
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return err
}

// ---------------------------------------------------------------- commands & scripts

func runMeasured(ctx context.Context, command, label string, dryRun bool) Location {
	if label == "" {
		label = command
	}
	if dryRun {
		return Location{Path: label, Note: "nopreview"}
	}
	drive := SystemDrive()
	before := freeSpace(drive)
	outText, err := runCommand(ctx, command)
	after := freeSpace(drive)
	l := Location{Path: label, Bytes: max(after-before, 0)}
	if err != nil {
		l.Note = strings.TrimSpace(err.Error() + " " + lastLine(outText))
	}
	return l
}

// runScriptMeasured runs a PowerShell script. Scripts may print lines
// "SIZESCOPE|<path>|<bytes>|<files>" to report what they removed; otherwise
// the free-space difference of the system drive is used. In preview mode the
// environment variable SIZESCOPE_DRYRUN=1 is set.
func runScriptMeasured(ctx context.Context, script string, dryRun, preview bool) ([]Location, bool) {
	if dryRun && !preview {
		// never run a script in preview mode unless it declares support for it
		return []Location{{Path: filepath.Base(script), Note: "nopreview"}}, false
	}
	drive := SystemDrive()
	before := freeSpace(drive)
	outText, err := runScript(ctx, script, dryRun)
	after := freeSpace(drive)
	locs := parseReport(outText)
	if len(locs) > 0 {
		if err != nil {
			locs = append(locs, Location{Path: filepath.Base(script), Note: err.Error()})
		}
		return locs, false
	}
	l := Location{Path: filepath.Base(script)}
	if dryRun {
		l.Note = "nopreview"
	} else {
		l.Bytes = max(after-before, 0)
	}
	if err != nil {
		l.Note = strings.TrimSpace(err.Error() + " " + lastLine(outText))
	}
	return []Location{l}, !dryRun
}

func parseReport(text string) []Location {
	var out []Location
	sc := bufio.NewScanner(strings.NewReader(text))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if !strings.HasPrefix(line, "SIZESCOPE|") {
			continue
		}
		f := strings.Split(line, "|")
		if len(f) < 3 {
			continue
		}
		l := Location{Path: f[1]}
		l.Bytes, _ = strconv.ParseInt(strings.TrimSpace(f[2]), 10, 64)
		if len(f) > 3 {
			l.Files, _ = strconv.ParseInt(strings.TrimSpace(f[3]), 10, 64)
		}
		out = append(out, l)
	}
	return out
}

func lastLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.LastIndexAny(s, "\r\n"); i >= 0 {
		s = s[i+1:]
	}
	if len(s) > 200 {
		s = s[:200]
	}
	return s
}

func recycleBin(dryRun bool) Location {
	l := Location{Path: "recyclebin"}
	bytes, files, err := recycleBinInfo()
	if err != nil {
		l.Note = err.Error()
		return l
	}
	l.Bytes, l.Files = bytes, files
	if !dryRun && files > 0 {
		if err := emptyRecycleBin(); err != nil {
			l.Note = err.Error()
			l.Bytes, l.Files = 0, 0
		}
	}
	return l
}

// SystemDrive returns the root of the Windows drive (or / elsewhere).
func SystemDrive() string {
	if d := os.Getenv("SystemDrive"); d != "" {
		return d + `\`
	}
	return string(filepath.Separator)
}
