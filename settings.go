package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Settings are stored next to the executable (portable). If that folder is
// not writable, the user config folder is used instead.
type Settings struct {
	Language      string   `json:"language"`      // auto | de | en
	Theme         string   `json:"theme"`         // system | light | dark
	Unit          string   `json:"unit"`          // auto | KB | MB | GB
	SizeMode      string   `json:"sizeMode"`      // size | alloc
	ShowFiles     bool     `json:"showFiles"`     // list files in the tree
	MapColor      string   `json:"mapColor"`      // type | depth
	Exclude       []string `json:"exclude"`       // name patterns skipped while scanning
	TopFiles      int      `json:"topFiles"`      // length of the largest-files list
	CsvSeparator  string   `json:"csvSeparator"`  // ; or ,
	ExportDepth   int      `json:"exportDepth"`   // 0 = all levels
	ExportFiles   bool     `json:"exportFiles"`   // include files in CSV export
	ConfirmDelete bool     `json:"confirmDelete"` // ask before moving to the recycle bin
	CloudFetch    bool     `json:"cloudFetch"`    // let OneDrive & co. download online-only folder listings
	OnlineFiles   string   `json:"onlineFiles"`   // skip | count: files that are only stored in the cloud
	AdminHint     bool     `json:"adminHint"`     // warn when running without administrator rights
	BackupRead    bool     `json:"backupRead"`    // as administrator, read protected folders via backup privilege
	Recent        []string `json:"recent"`
}

func defaultSettings() Settings {
	return Settings{
		Language: "auto", Theme: "system", Unit: "auto", SizeMode: "size", ShowFiles: true,
		MapColor: "type", OnlineFiles: "skip", AdminHint: true, BackupRead: true, TopFiles: 200, CsvSeparator: ";", ExportDepth: 0, ConfirmDelete: true,
	}
}

const settingsName = "SizeScope.settings.json"

func settingsCandidates() []string {
	var c []string
	if exe, err := os.Executable(); err == nil {
		c = append(c, filepath.Join(filepath.Dir(exe), settingsName))
	}
	if dir, err := os.UserConfigDir(); err == nil {
		c = append(c, filepath.Join(dir, "SizeScope", settingsName))
	}
	return c
}

func loadSettings() Settings {
	s := defaultSettings()
	for _, p := range settingsCandidates() {
		if b, err := os.ReadFile(p); err == nil {
			_ = json.Unmarshal(b, &s)
			break
		}
	}
	if s.TopFiles <= 0 {
		s.TopFiles = 200
	}
	return s
}

func saveSettings(s Settings) error {
	b, _ := json.MarshalIndent(s, "", "  ")
	var last error
	for _, p := range settingsCandidates() {
		_ = os.MkdirAll(filepath.Dir(p), 0o755)
		if last = os.WriteFile(p, b, 0o644); last == nil {
			return nil
		}
	}
	return last
}
