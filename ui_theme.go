package main

import (
	"image"
	"image/color"
	"strconv"
	"strings"
	"time"

	"gioui.org/font"
	"gioui.org/font/gofont"
	"gioui.org/font/opentype"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

// Palette holds the interface colours of one theme.
type Palette struct {
	Bg, Surface, Raised, Ink, Muted, Faint, Line, LineSoft color.NRGBA
	Accent, AccentInk, AccentSoft, Sel, Hover, BarTrack    color.NRGBA
	Danger, Warn, Folder, Shadow, Backdrop, MapFolderInk   color.NRGBA
	WarnSoft                                               color.NRGBA
	MapFolderBase                                          [3]uint8 // rgb at depth 0
	MapFolderStep                                          int      // lightness change per depth
}

func rgb(h uint32) color.NRGBA {
	return color.NRGBA{R: uint8(h >> 16), G: uint8(h >> 8), B: uint8(h), A: 255}
}
func rgba(h uint32, a uint8) color.NRGBA { c := rgb(h); c.A = a; return c }

var lightPal = Palette{
	Bg: rgb(0xe4e9ee), Surface: rgb(0xf6f8fa), Raised: rgb(0xffffff), Ink: rgb(0x15202b),
	Muted: rgb(0x5d6c79), Faint: rgb(0x8d99a4), Line: rgb(0xcfd7de), LineSoft: rgb(0xe3e8ed),
	Accent: rgb(0x1d6d89), AccentInk: rgb(0xffffff), AccentSoft: rgb(0xd3e6ee), Sel: rgb(0xcfe3ec),
	Hover: rgb(0xedf2f5), BarTrack: rgb(0xe1e7ec), Danger: rgb(0xb93a2c), Warn: rgb(0xa8670f),
	Folder: rgb(0xc9a24a), Shadow: rgba(0x15202b, 40), Backdrop: rgba(0x0a1016, 110),
	MapFolderInk: rgba(0x15202b, 200), MapFolderBase: [3]uint8{0xd9, 0xdf, 0xe5}, MapFolderStep: -10,
	WarnSoft: rgb(0xfbefd8),
}

var darkPal = Palette{
	Bg: rgb(0x11161c), Surface: rgb(0x182028), Raised: rgb(0x1f2932), Ink: rgb(0xe2e8ee),
	Muted: rgb(0x8995a1), Faint: rgb(0x5e6a76), Line: rgb(0x2a343e), LineSoft: rgb(0x212b34),
	Accent: rgb(0x52b4cf), AccentInk: rgb(0x0b1a20), AccentSoft: rgb(0x1d3b47), Sel: rgb(0x20404d),
	Hover: rgb(0x1d2730), BarTrack: rgb(0x26313b), Danger: rgb(0xe2715f), Warn: rgb(0xe0a54b),
	Folder: rgb(0xd8b45d), Shadow: rgba(0x000000, 110), Backdrop: rgba(0x000000, 130),
	MapFolderInk: rgba(0xe2e8ee, 215), MapFolderBase: [3]uint8{0x1e, 0x27, 0x30}, MapFolderStep: 7,
	WarnSoft: rgb(0x33291a),
}

// file type colours, shared by both themes
var catColors = map[string]color.NRGBA{
	"video": rgb(0xc4573f), "image": rgb(0xd69a2d), "audio": rgb(0x8b6cc4), "archive": rgb(0x3c8f7f),
	"doc": rgb(0x4a7cc6), "code": rgb(0x6aa057), "data": rgb(0xb5638b), "system": rgb(0x5d7590),
	"other": rgb(0xa5aca3),
}

var categories = []string{"video", "image", "audio", "archive", "doc", "code", "data", "system", "other"}

var extCategory = func() map[string]string {
	m := map[string]string{}
	for cat, list := range extGroups {
		for _, e := range strings.Fields(list) {
			m[e] = cat
		}
	}
	return m
}()

func catOf(ext string) string {
	if c, ok := extCategory[ext]; ok {
		return c
	}
	return "other"
}

func mapFolderColor(p *Palette, depth int) color.NRGBA {
	if depth > 6 {
		depth = 6
	}
	adj := func(v uint8) uint8 {
		n := int(v) + p.MapFolderStep*depth
		if n < 0 {
			n = 0
		}
		if n > 255 {
			n = 255
		}
		return uint8(n)
	}
	b := p.MapFolderBase
	return color.NRGBA{R: adj(b[0]), G: adj(b[1]), B: adj(b[2]), A: 255}
}

// ---------------------------------------------------------------- fonts

func loadFonts() ([]font.FontFace, font.Typeface) {
	var faces []font.FontFace
	for _, f := range systemFontFiles() {
		coll, err := opentype.ParseCollection(f.data)
		if err != nil || len(coll) == 0 {
			continue
		}
		face := coll[0]
		face.Font = font.Font{Typeface: "UI", Weight: f.weight}
		faces = append(faces, face)
	}
	if len(faces) == 0 {
		return gofont.Collection(), ""
	}
	// keep Go fonts as fallback for glyphs the system font lacks
	return append(faces, gofont.Collection()...), "UI"
}

type fontFile struct {
	data   []byte
	weight font.Weight
}

// ---------------------------------------------------------------- drawing helpers

func fill(gtx layout.Context, r image.Rectangle, c color.NRGBA) {
	paint.FillShape(gtx.Ops, c, clip.Rect(r).Op())
}

func fillRR(gtx layout.Context, r image.Rectangle, rad int, c color.NRGBA) {
	paint.FillShape(gtx.Ops, c, clip.UniformRRect(r, rad).Op(gtx.Ops))
}

func strokeRR(gtx layout.Context, r image.Rectangle, rad int, width float32, c color.NRGBA) {
	paint.FillShape(gtx.Ops, c, clip.Stroke{Path: clip.UniformRRect(r, rad).Path(gtx.Ops), Width: width}.Op())
}

func strokeRect(gtx layout.Context, r image.Rectangle, width int, c color.NRGBA) {
	fill(gtx, image.Rect(r.Min.X, r.Min.Y, r.Max.X, r.Min.Y+width), c)
	fill(gtx, image.Rect(r.Min.X, r.Max.Y-width, r.Max.X, r.Max.Y), c)
	fill(gtx, image.Rect(r.Min.X, r.Min.Y, r.Min.X+width, r.Max.Y), c)
	fill(gtx, image.Rect(r.Max.X-width, r.Min.Y, r.Max.X, r.Max.Y), c)
}

// within runs f with the origin moved to r.Min, exact constraints and a clip to r.
func within(gtx layout.Context, r image.Rectangle, f func(gtx layout.Context)) {
	if r.Dx() <= 0 || r.Dy() <= 0 {
		return
	}
	st := op.Offset(r.Min).Push(gtx.Ops)
	cl := clip.Rect(image.Rectangle{Max: r.Size()}).Push(gtx.Ops)
	g := gtx
	g.Constraints = layout.Exact(r.Size())
	f(g)
	cl.Pop()
	st.Pop()
}

// Text draws a single line, truncated with "…", vertically centred in r.
func (u *UI) text(gtx layout.Context, r image.Rectangle, s string, size unit.Sp, col color.NRGBA, w font.Weight, align text.Alignment) int {
	if r.Dx() <= 2 || s == "" {
		return 0
	}
	st := op.Offset(r.Min).Push(gtx.Ops)
	g := gtx
	g.Constraints = layout.Constraints{Min: image.Pt(r.Dx(), 0), Max: image.Pt(r.Dx(), 1<<16)}
	if align == text.Start {
		// natural width, so callers can place things right after the text
		g.Constraints.Min.X = 0
	}
	m := op.Record(gtx.Ops)
	l := material.Label(u.th, size, s)
	l.Color = col
	l.Font.Weight = w
	l.MaxLines = 1
	l.Alignment = align
	d := l.Layout(g)
	call := m.Stop()
	o := op.Offset(image.Pt(0, (r.Dy()-d.Size.Y)/2)).Push(gtx.Ops)
	call.Add(gtx.Ops)
	o.Pop()
	st.Pop()
	return d.Size.X
}

// textWidth measures a single line of text.
func (u *UI) textWidth(gtx layout.Context, s string, size unit.Sp, w font.Weight) int {
	g := gtx
	g.Constraints = layout.Constraints{Max: image.Pt(1<<16, 1<<16)}
	m := op.Record(gtx.Ops)
	l := material.Label(u.th, size, s)
	l.Font.Weight = w
	l.MaxLines = 1
	d := l.Layout(g)
	m.Stop()
	return d.Size.X
}

// wrapText draws wrapping text at pt within width; returns the height used.
func (u *UI) wrapText(gtx layout.Context, pt image.Point, width int, s string, size unit.Sp, col color.NRGBA) int {
	st := op.Offset(pt).Push(gtx.Ops)
	g := gtx
	g.Constraints = layout.Constraints{Max: image.Pt(width, 1<<16)}
	l := material.Label(u.th, size, s)
	l.Color = col
	d := l.Layout(g)
	st.Pop()
	return d.Size.Y
}

func (u *UI) icon(gtx layout.Context, ic *widget.Icon, r image.Rectangle, col color.NRGBA) {
	within(gtx, r, func(gtx layout.Context) { ic.Layout(gtx, col) })
}

func (u *UI) dp(gtx layout.Context, v float32) int { return gtx.Dp(unit.Dp(v)) }

// ---------------------------------------------------------------- formatting

func (u *UI) num(n int64) string {
	s := strconv.FormatInt(n, 10)
	neg := strings.HasPrefix(s, "-")
	if neg {
		s = s[1:]
	}
	sep := ","
	if u.lang == "de" {
		sep = "."
	}
	var b strings.Builder
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteString(sep)
		}
		b.WriteRune(c)
	}
	if neg {
		return "-" + b.String()
	}
	return b.String()
}

func (u *UI) dec1(v float64) string {
	s := strconv.FormatFloat(v, 'f', 1, 64)
	if u.lang == "de" {
		s = strings.Replace(s, ".", ",", 1)
	}
	return s
}

var units = []string{"B", "KB", "MB", "GB", "TB", "PB"}

func (u *UI) size(b int64) string {
	if fixed := u.settings.Unit; fixed != "" && fixed != "auto" {
		for i, un := range units {
			if un == fixed {
				v := float64(b)
				for j := 0; j < i; j++ {
					v /= 1024
				}
				if v >= 100 {
					return u.num(int64(v+0.5)) + " " + un
				}
				return u.dec1(v) + " " + un
			}
		}
	}
	if b < 1024 && b > -1024 {
		return u.num(b) + " B"
	}
	v, i := float64(b), 0
	for (v >= 1024 || v <= -1024) && i < len(units)-1 {
		v /= 1024
		i++
	}
	if v >= 100 {
		return u.num(int64(v+0.5)) + " " + units[i]
	}
	return u.dec1(v) + " " + units[i]
}

func (u *UI) pct(p float64) string {
	if p >= 99.95 {
		return "100 %"
	}
	return u.dec1(p) + " %"
}

func (u *UI) date(unix int64) string {
	if unix <= 0 {
		return ""
	}
	t := time.Unix(unix, 0)
	if u.lang == "de" {
		return t.Format("02.01.06, 15:04")
	}
	return t.Format("1/2/06 3:04 PM")
}

func dur(d time.Duration) string {
	s := int(d.Seconds())
	m, h := s/60, s/3600
	if h > 0 {
		return strconv.Itoa(h) + ":" + pad2(m%60) + ":" + pad2(s%60)
	}
	return strconv.Itoa(m) + ":" + pad2(s%60)
}

func pad2(n int) string {
	if n < 10 {
		return "0" + strconv.Itoa(n)
	}
	return strconv.Itoa(n)
}

func (u *UI) t(key string, kv ...string) string {
	s, ok := i18n[u.lang][key]
	if !ok {
		if s, ok = i18n["en"][key]; !ok {
			s = key
		}
	}
	for i := 0; i+1 < len(kv); i += 2 {
		s = strings.ReplaceAll(s, "{"+kv[i]+"}", kv[i+1])
	}
	return s
}
