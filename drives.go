package main

// Drive is a volume shown on the start screen.
type Drive struct {
	Path  string
	Label string
	FS    string
	Kind  string // fixed | removable | network | cdrom | ram | other
	Total int64
	Free  int64
}
