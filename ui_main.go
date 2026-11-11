package main

import (
	"context"
	"fmt"
	"image"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"gioui.org/app"
	"gioui.org/font"
	"gioui.org/io/clipboard"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"sizescope/scanner"
)

// appVersion is overridden at build time via -ldflags "-X main.appVersion=...".
var appVersion = "2.0.2"

func main() {
	u := newUI()
	go func() {
		w := new(app.Window)
		w.Option(app.Title("SizeScope"), app.Size(unit.Dp(1360), unit.Dp(860)), app.MinSize(unit.Dp(900), unit.Dp(560)))
		u.win = w
		if err := u.run(w); err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
		os.Exit(0)
	}()
	app.Main()
}

type flatItem struct {
	row        scanner.Row
	depth      int
	parentSize int64
}

// UI holds the whole application state. It is only touched from the window
// goroutine, except for sideState which background queries fill in.
type UI struct {
	win      *app.Window
	hwnd     uintptr
	th       *material.Theme
	pal      Palette
	dark     bool
	lang     string
	settings Settings
	drives   []Drive
	admin    bool
	startArg string

	tree *scanner.Tree

	// tree view
	expanded  map[string]bool
	flat      []flatItem
	flatAt    time.Time
	flatDirty bool
	selected  string
	focusDir  string
	sortBy    string
	sortDesc  bool
	treeList  VList
	treeHead  int8
	treeFocus int8
	treeRect  image.Rectangle
	rowH      int
	headH     int
	lastPress struct {
		row int
		at  time.Duration
	}
	wantReveal string

	// side panel
	tab       int
	topFilter string
	side      sideState
	mapView   MapView
	typesList VList
	topList   VList
	sideAt    time.Time
	crumbs    []string
	crumbBtns []widget.Clickable
	tabBtns   [4]widget.Clickable
	chipBtn   widget.Clickable

	// widgets
	pathEd                                                            widget.Editor
	btnScan, btnStop, btnPick, btnDrives, btnExport, btnAdmin, btnSet widget.Clickable
	btnErrors                                                         widget.Clickable
	btnBannerAdmin, btnBannerClose                                    widget.Clickable
	bannerClosed                                                      bool
	driveBtns                                                         []widget.Clickable
	recentBtns                                                        []widget.Clickable
	split                                                             float32
	splitTag                                                          int8
	splitDrag                                                         bool

	// overlays
	menu     *Menu
	popover  *Menu
	tip      *tipInfo
	toasts   []toast
	dlg      *settingsDlg
	clip     string
	focusReq bool
	uiMu     sync.Mutex
	pending  []func()
}

func newUI() *UI {
	u := &UI{
		settings: loadSettings(), expanded: map[string]bool{}, sortBy: "size", sortDesc: true, split: 0.6,
	}
	u.treeList.Hover, u.typesList.Hover, u.topList.Hover = -1, -1, -1
	faces, typeface := loadFonts()
	u.th = material.NewTheme()
	u.th.Shaper = text.NewShaper(text.WithCollection(faces), text.NoSystemFonts())
	u.th.Face = typeface
	u.th.TextSize = 13
	u.pathEd.SingleLine = true
	u.pathEd.Submit = true
	u.admin = isAdmin()
	// disconnected network drives can take seconds to answer: load in the background
	u.goSafe(func() {
		d := listDrives()
		u.post(func() { u.drives = d })
	})
	if len(os.Args) > 1 && !strings.HasPrefix(os.Args[1], "-") {
		u.startArg = os.Args[1]
	}
	u.applySettings()
	return u
}

// goSafe runs f in the background; a panic is reported as an error message
// instead of terminating the program.
func (u *UI) goSafe(f func()) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				msg := fmt.Sprint(r)
				u.post(func() { u.toast(u.t("eGeneric", "msg", msg), true) })
			}
		}()
		f()
	}()
}

// post runs f on the window goroutine before the next frame (safe from any goroutine).
func (u *UI) post(f func()) {
	u.uiMu.Lock()
	u.pending = append(u.pending, f)
	u.uiMu.Unlock()
	if u.win != nil {
		u.win.Invalidate()
	}
}

func (u *UI) applySettings() {
	l := u.settings.Language
	if l == "" || l == "auto" {
		l = systemLanguage()
	}
	if _, ok := i18n[l]; !ok {
		l = "en"
	}
	u.lang = l
	switch u.settings.Theme {
	case "dark":
		u.dark = true
	case "light":
		u.dark = false
	default:
		u.dark = systemDarkMode()
	}
	if u.dark {
		u.pal = darkPal
	} else {
		u.pal = lightPal
	}
	u.th.Palette = material.Palette{Bg: u.pal.Raised, Fg: u.pal.Ink, ContrastBg: u.pal.Accent, ContrastFg: u.pal.AccentInk}
	if u.hwnd != 0 {
		setTitleBarDark(u.hwnd, u.dark)
	}
	u.mapView.dirty = true
	u.flatDirty = true
}

func (u *UI) run(w *app.Window) error {
	var ops op.Ops
	for {
		switch e := w.Event().(type) {
		case app.DestroyEvent:
			return e.Err
		case app.FrameEvent:
			gtx := app.NewContext(&ops, e)
			u.frame(gtx)
			e.Frame(gtx.Ops)
		default:
			if h := windowHandle(e); h != 0 {
				u.hwnd = h
				setTitleBarDark(h, u.dark)
			}
		}
	}
}

// ---------------------------------------------------------------- frame

func (u *UI) frame(gtx layout.Context) {
	u.uiMu.Lock()
	pend := u.pending
	u.pending = nil
	u.uiMu.Unlock()
	for _, f := range pend {
		f()
	}
	if u.startArg != "" {
		p := u.startArg
		u.startArg = ""
		u.startScan(p)
	}
	if u.clip != "" {
		gtx.Execute(clipboard.WriteCmd{Type: "application/text", Data: io.NopCloser(strings.NewReader(u.clip))})
		u.clip = ""
	}
	u.globalKeys(gtx)

	t := u.tree
	running := t != nil && t.Running()
	if running {
		gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(450 * time.Millisecond)})
	}
	if t != nil && (u.flatDirty || (running && time.Since(u.flatAt) > 400*time.Millisecond)) {
		u.rebuildFlat()
	}
	if t != nil && running && time.Since(u.sideAt) > 2500*time.Millisecond {
		u.requestSide()
	}
	if t != nil && !running && u.side.lastRunning {
		// final refresh once the scan has finished
		u.side.lastRunning = false
		u.rebuildFlat()
		u.requestSide()
	}
	if running {
		u.side.lastRunning = true
	}

	full := gtx.Constraints.Max
	fill(gtx, image.Rectangle{Max: full}, u.pal.Bg)
	tbH, sbH := u.dp(gtx, 50), u.dp(gtx, 28)
	u.tip = nil
	u.toolbar(gtx, image.Rect(0, 0, full.X, tbH))
	top := tbH
	if u.showAdminBanner() {
		bh := u.dp(gtx, 40)
		u.adminBanner(gtx, image.Rect(0, top, full.X, top+bh))
		top += bh
	}
	body := image.Rect(0, top, full.X, full.Y-sbH)
	if t == nil {
		u.welcome(gtx, body)
	} else {
		u.workspace(gtx, body)
	}
	u.statusbar(gtx, image.Rect(0, full.Y-sbH, full.X, full.Y))

	if u.popover != nil {
		m := u.menu
		u.menu = u.popover
		u.layoutMenu(gtx)
		u.popover = u.menu
		u.menu = m
	}
	if u.menu != nil {
		u.layoutMenu(gtx)
	}
	if u.dlg != nil {
		u.layoutSettings(gtx)
	}
	if u.menu == nil && u.dlg == nil {
		u.layoutTip(gtx)
	}
	u.layoutToasts(gtx)
}

func (u *UI) globalKeys(gtx layout.Context) {
	for {
		ev, ok := gtx.Event(key.Filter{Name: key.NameEscape}, key.Filter{Name: key.NameF5})
		if !ok {
			break
		}
		e, ok := ev.(key.Event)
		if !ok || e.State != key.Press {
			continue
		}
		switch e.Name {
		case key.NameEscape:
			u.menu, u.popover, u.dlg = nil, nil, nil
		case key.NameF5:
			if u.tree != nil && u.selected != "" {
				u.rescan(u.dirKeyOf(u.selected))
			}
		}
	}
}

// ---------------------------------------------------------------- toolbar

func (u *UI) toolbar(gtx layout.Context, r image.Rectangle) {
	fill(gtx, r, u.pal.Surface)
	fill(gtx, image.Rect(r.Min.X, r.Max.Y-1, r.Max.X, r.Max.Y), u.pal.Line)
	d := func(v float32) int { return u.dp(gtx, v) }
	h := d(32)
	y := r.Min.Y + (r.Dy()-h)/2

	// brand mark: three tiles in the treemap colours
	bx := d(12)
	s := d(22)
	by := r.Min.Y + (r.Dy()-s)/2
	fillRR(gtx, image.Rect(bx, by, bx+s*11/24, by+s), d(2), catColors["video"])
	fillRR(gtx, image.Rect(bx+s*13/24, by, bx+s, by+s*11/24), d(2), catColors["doc"])
	fillRR(gtx, image.Rect(bx+s*13/24, by+s*13/24, bx+s, by+s), d(2), catColors["image"])

	// path field
	px := bx + s + d(14)
	pw := r.Dx() / 2
	if pw > d(560) {
		pw = d(560)
	}
	pr := image.Rect(px, y, px+pw, y+h)
	fillRR(gtx, pr, d(6), u.pal.Raised)
	edFocused := gtx.Focused(&u.pathEd)
	bc := u.pal.Line
	if edFocused {
		bc = u.pal.Accent
	}
	strokeRR(gtx, pr, d(6), 1, bc)
	for {
		ev, ok := u.pathEd.Update(gtx)
		if !ok {
			break
		}
		if _, ok := ev.(widget.SubmitEvent); ok {
			u.startScan(u.pathEd.Text())
		}
	}
	within(gtx, image.Rect(pr.Min.X+d(10), pr.Min.Y, pr.Max.X-d(64), pr.Max.Y), func(gtx layout.Context) {
		gtx.Constraints.Min.Y = 0
		m := op.Record(gtx.Ops)
		e := material.Editor(u.th, &u.pathEd, u.t("pathPlaceholder"))
		e.Color, e.HintColor = u.pal.Ink, u.pal.Faint
		e.TextSize = 13
		dims := e.Layout(gtx)
		call := m.Stop()
		o := op.Offset(image.Pt(0, (gtx.Constraints.Max.Y-dims.Size.Y)/2)).Push(gtx.Ops)
		call.Add(gtx.Ops)
		o.Pop()
	})
	ib := d(28)
	if u.iconButton(gtx, &u.btnPick, image.Rect(pr.Max.X-2*ib-d(4), y+d(2), pr.Max.X-ib-d(4), y+d(2)+ib), ic.pick, u.pal.Muted, true) {
		u.pickFolder()
	}
	if u.iconButton(gtx, &u.btnDrives, image.Rect(pr.Max.X-ib-d(2), y+d(2), pr.Max.X-d(2), y+d(2)+ib), ic.drive, u.pal.Muted, true) {
		u.openDrivePopover(image.Pt(pr.Max.X-d(250), pr.Max.Y+d(6)))
	}

	running := u.tree != nil && u.tree.Running()
	bxr := image.Rect(pr.Max.X+d(8), y, pr.Max.X+d(8)+d(112), y+h)
	if running {
		if u.button(gtx, &u.btnStop, bxr, u.t("stop"), ic.stop, btnPlain) {
			u.tree.Cancel()
		}
	} else if u.button(gtx, &u.btnScan, bxr, u.t("scan"), ic.play, btnPrimary) {
		u.startScan(u.pathEd.Text())
	}

	// right side
	x := r.Max.X - d(8) - h
	if u.iconButton(gtx, &u.btnSet, image.Rect(x, y, x+h, y+h), ic.gear, u.pal.Muted, true) {
		u.openSettings()
	}
	if !u.admin && platformName == "windows" {
		x -= h + d(2)
		if u.iconButton(gtx, &u.btnAdmin, image.Rect(x, y, x+h, y+h), ic.shield, u.pal.Warn, true) {
			u.restartAdmin()
		}
	}
	x -= h + d(2)
	if u.iconButton(gtx, &u.btnExport, image.Rect(x, y, x+h, y+h), ic.export, u.pal.Muted, u.tree != nil && !running) {
		u.export()
	}
}

func (u *UI) openDrivePopover(at image.Point) {
	var items []menuItem
	for _, d := range u.drives {
		d := d
		lbl := strings.TrimSuffix(d.Path, `\`)
		if name := d.Label; name != "" {
			lbl += "  " + name
		}
		hint := ""
		if d.Total > 0 {
			hint = u.t("driveFree", "free", u.size(d.Free))
		}
		items = append(items, menuItem{icon: driveIcon(d.Kind), label: lbl, hint: hint, action: func() { u.startScan(d.Path) }})
	}
	u.popover = &Menu{at: at, items: items, btns: make([]widget.Clickable, len(items))}
}

// ---------------------------------------------------------------- welcome

func (u *UI) welcome(gtx layout.Context, r image.Rectangle) {
	d := func(v float32) int { return u.dp(gtx, v) }
	w := d(760)
	if w > r.Dx()-d(40) {
		w = r.Dx() - d(40)
	}
	x := r.Min.X + (r.Dx()-w)/2
	y := r.Min.Y + d(52)
	u.text(gtx, image.Rect(x, y, x+w, y+d(38)), u.t("welcomeTitle"), 26, u.pal.Ink, font.SemiBold, text.Start)
	y += d(44)
	y += u.wrapText(gtx, image.Pt(x, y), w*8/10, u.t("welcomeLead"), 15, u.pal.Muted) + d(24)

	if len(u.driveBtns) != len(u.drives) {
		u.driveBtns = make([]widget.Clickable, len(u.drives))
	}
	cardH := d(76)
	for i, dr := range u.drives {
		dr := dr
		cr := image.Rect(x, y, x+w, y+cardH)
		if u.driveBtns[i].Clicked(gtx) {
			u.startScan(dr.Path)
		}
		within(gtx, cr, func(gtx layout.Context) {
			u.driveBtns[i].Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				sz := gtx.Constraints.Min
				box := image.Rectangle{Max: sz}
				fillRR(gtx, box, d(10), u.pal.Surface)
				bc := u.pal.Line
				if u.driveBtns[i].Hovered() {
					bc = u.pal.Accent
				}
				strokeRR(gtx, box, d(10), 1, bc)
				u.icon(gtx, driveIcon(dr.Kind), image.Rect(d(18), (sz.Y-d(30))/2, d(48), (sz.Y+d(30))/2), u.pal.Muted)
				tx := d(66)
				goW := d(90)
				name := strings.TrimSuffix(dr.Path, `\`)
				nw := u.text(gtx, image.Rect(tx, d(10), sz.X-goW, d(32)), name, 15, u.pal.Ink, font.SemiBold, text.Start)
				lbl := dr.Label
				if lbl == "" {
					lbl = u.t("kind_" + dr.Kind)
				}
				if dr.FS != "" {
					lbl += " (" + dr.FS + ")"
				}
				u.text(gtx, image.Rect(tx+nw+d(8), d(10), sz.X-goW, d(32)), lbl, 13, u.pal.Muted, font.Normal, text.Start)
				switch {
				case dr.Total > 0:
					used := max(dr.Total-dr.Free, 0)
					bar := image.Rect(tx, d(38), sz.X-goW, d(46))
					fillRR(gtx, bar, d(4), u.pal.BarTrack)
					fw := min(int(float64(bar.Dx())*float64(used)/float64(dr.Total)), bar.Dx())
					col := u.pal.Accent
					if float64(used)/float64(dr.Total) > 0.9 {
						col = u.pal.Danger
					}
					fillRR(gtx, image.Rect(bar.Min.X, bar.Min.Y, bar.Min.X+fw, bar.Max.Y), d(4), col)
					meta := u.t("driveUsed", "used", u.size(used), "total", u.size(dr.Total)) + ", " + u.t("driveFree", "free", u.size(dr.Free))
					u.text(gtx, image.Rect(tx, d(50), sz.X-goW, d(68)), meta, 12, u.pal.Muted, font.Normal, text.Start)
				case dr.Free > 0:
					u.text(gtx, image.Rect(tx, d(38), sz.X-goW, d(56)), u.t("driveFree", "free", u.size(dr.Free)), 12, u.pal.Muted, font.Normal, text.Start)
				default:
					u.text(gtx, image.Rect(tx, d(38), sz.X-goW, d(56)), u.t("driveOffline"), 12, u.pal.Faint, font.Normal, text.Start)
				}
				u.text(gtx, image.Rect(sz.X-goW, 0, sz.X-d(18), sz.Y), u.t("driveScan"), 13, u.pal.Accent, font.SemiBold, text.End)
				pointer.CursorPointer.Add(gtx.Ops)
				return layout.Dimensions{Size: sz}
			})
		})
		y += cardH + d(8)
	}
	rec := u.settings.Recent
	if len(rec) > 0 {
		y += d(20)
		u.text(gtx, image.Rect(x, y, x+w, y+d(22)), u.t("recent"), 14, u.pal.Muted, font.SemiBold, text.Start)
		y += d(28)
		if len(u.recentBtns) != len(rec) {
			u.recentBtns = make([]widget.Clickable, len(rec))
		}
		for i, p := range rec {
			p := p
			if u.recentBtns[i].Clicked(gtx) {
				u.startScan(p)
			}
			rr := image.Rect(x, y, x+w, y+d(30))
			within(gtx, rr, func(gtx layout.Context) {
				u.recentBtns[i].Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					sz := gtx.Constraints.Min
					if u.recentBtns[i].Hovered() {
						fillRR(gtx, image.Rectangle{Max: sz}, d(6), u.pal.Hover)
					}
					u.text(gtx, image.Rect(d(8), 0, sz.X-d(8), sz.Y), p, 13, u.pal.Accent, font.Normal, text.Start)
					pointer.CursorPointer.Add(gtx.Ops)
					return layout.Dimensions{Size: sz}
				})
			})
			y += d(30)
		}
	}
}

// ---------------------------------------------------------------- workspace

func (u *UI) workspace(gtx layout.Context, r image.Rectangle) {
	d := func(v float32) int { return u.dp(gtx, v) }
	scanH := d(48)
	u.scanbar(gtx, image.Rect(r.Min.X, r.Min.Y, r.Max.X, r.Min.Y+scanH))
	body := image.Rect(r.Min.X, r.Min.Y+scanH, r.Max.X, r.Max.Y)

	// splitter drag
	for _, e := range pointerEvents(gtx, &u.splitTag, pointer.Press|pointer.Drag|pointer.Release) {
		switch e.Kind {
		case pointer.Press:
			u.splitDrag = true
		case pointer.Drag:
			if u.splitDrag {
				x := float32(u.splitX(body)) + e.Position.X
				f := (x - float32(body.Min.X)) / float32(body.Dx())
				if f < 0.25 {
					f = 0.25
				}
				if f > 0.8 {
					f = 0.8
				}
				u.split = f
				u.mapView.dirty = true
			}
		case pointer.Release:
			u.splitDrag = false
		}
	}
	sx := u.splitX(body)
	sw := d(6)
	u.treeView(gtx, image.Rect(body.Min.X, body.Min.Y, sx, body.Max.Y))
	sr := image.Rect(sx, body.Min.Y, sx+sw, body.Max.Y)
	fill(gtx, sr, u.pal.Bg)
	fill(gtx, image.Rect(sr.Min.X, sr.Min.Y, sr.Min.X+1, sr.Max.Y), u.pal.Line)
	fill(gtx, image.Rect(sr.Max.X-1, sr.Min.Y, sr.Max.X, sr.Max.Y), u.pal.Line)
	if u.splitDrag {
		fill(gtx, sr.Inset(1), u.pal.AccentSoft)
	}
	within(gtx, sr, func(gtx layout.Context) {
		hitArea(gtx, image.Rectangle{Max: sr.Size()}, &u.splitTag, pointer.CursorColResize)
	})
	u.sidePanel(gtx, image.Rect(sx+sw, body.Min.Y, body.Max.X, body.Max.Y))
}

func (u *UI) splitX(body image.Rectangle) int {
	return body.Min.X + int(float32(body.Dx())*u.split)
}

func (u *UI) scanbar(gtx layout.Context, r image.Rectangle) {
	d := func(v float32) int { return u.dp(gtx, v) }
	fill(gtx, r, u.pal.Surface)
	fill(gtx, image.Rect(r.Min.X, r.Max.Y-1, r.Max.X, r.Max.Y), u.pal.Line)
	t := u.tree
	st := t.Stats()
	running := t.Running()
	volW := d(440)
	if volW > r.Dx()/2 {
		volW = r.Dx() / 2
	}
	x := r.Min.X + d(14)
	nw := u.text(gtx, image.Rect(x, r.Min.Y, r.Max.X-volW-d(260), r.Max.Y), t.RootPath, 16, u.pal.Ink, font.SemiBold, text.Start)
	x += nw + d(14)
	var state string
	col := u.pal.Muted
	if running {
		state = u.t("scanning") + "  " + dur(st.Elapsed)
		col = u.pal.Accent
		dot := d(8)
		cy := r.Min.Y + r.Dy()/2
		fillRR(gtx, image.Rect(x, cy-dot/2, x+dot, cy+dot/2), dot/2, col)
		x += dot + d(7)
	} else if st.Canceled {
		state = u.t("scanCanceled", "t", dur(st.Elapsed))
	} else {
		state = u.t("scanDone", "t", dur(st.Elapsed))
	}
	u.text(gtx, image.Rect(x, r.Min.Y, x+d(240), r.Max.Y), state, 13, col, font.Normal, text.Start)

	tot, free := diskUsage(t.RootPath)
	if tot > 0 {
		used := max(tot-free, 0)
		scanned := t.Root.Size()
		if scanned > used {
			scanned = used
		}
		vr := image.Rect(r.Max.X-volW-d(14), r.Min.Y, r.Max.X-d(14), r.Max.Y)
		txt := u.t("volText", "scanned", u.size(t.Root.Size()), "used", u.size(used), "total", u.size(tot))
		tw := u.textWidth(gtx, txt, 12, font.Normal)
		barW := vr.Dx() - tw - d(12)
		if barW < d(100) {
			barW = d(100)
		}
		cy := r.Min.Y + r.Dy()/2
		bar := image.Rect(vr.Min.X, cy-d(5), vr.Min.X+barW, cy+d(5))
		fillRR(gtx, bar, d(5), u.pal.BarTrack)
		sw := min(int(float64(barW)*float64(max(scanned, 0))/float64(tot)), barW)
		ow := min(int(float64(barW)*float64(max(used-scanned, 0))/float64(tot)), barW-sw)
		within(gtx, bar, func(gtx layout.Context) {
			fillRR(gtx, image.Rect(0, 0, bar.Dx(), bar.Dy()), d(5), u.pal.BarTrack)
			fill(gtx, image.Rect(0, 0, sw, bar.Dy()), u.pal.Accent)
			oc := u.pal.Faint
			oc.A = 140
			fill(gtx, image.Rect(sw, 0, sw+ow, bar.Dy()), oc)
		})
		u.text(gtx, image.Rect(bar.Max.X+d(12), r.Min.Y, vr.Max.X, r.Max.Y), txt, 12, u.pal.Muted, font.Normal, text.Start)
	}
}

func (u *UI) statusbar(gtx layout.Context, r image.Rectangle) {
	d := func(v float32) int { return u.dp(gtx, v) }
	fill(gtx, r, u.pal.Surface)
	fill(gtx, image.Rect(r.Min.X, r.Min.Y, r.Max.X, r.Min.Y+1), u.pal.Line)
	t := u.tree
	if t == nil {
		return
	}
	st := t.Stats()
	x := r.Min.X + d(12)
	seg := func(s string, col interface{}) int {
		w := u.textWidth(gtx, s, 12, font.Normal) + d(4)
		u.text(gtx, image.Rect(x, r.Min.Y, x+w, r.Max.Y), s, 12, u.pal.Muted, font.Normal, text.Start)
		x += w + d(18)
		return w
	}
	seg(u.t("stFiles", "n", u.num(st.Files)), nil)
	seg(u.t("stDirs", "n", u.num(st.Dirs)), nil)
	if st.Errors > 0 {
		s := u.t("stErrors", "n", u.num(st.Errors))
		w := u.textWidth(gtx, s, 12, font.Normal) + d(4)
		br := image.Rect(x, r.Min.Y, x+w, r.Max.Y)
		if u.btnErrors.Clicked(gtx) && !u.admin && platformName == "windows" {
			u.restartAdmin()
		}
		within(gtx, br, func(gtx layout.Context) {
			u.btnErrors.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				sz := gtx.Constraints.Min
				u.text(gtx, image.Rectangle{Max: sz}, s, 12, u.pal.Warn, font.Normal, text.Start)
				if u.btnErrors.Hovered() {
					fill(gtx, image.Rect(0, sz.Y-d(6), sz.X-d(4), sz.Y-d(5)), u.pal.Warn)
					if !u.admin && platformName == "windows" {
						u.tip = &tipInfo{at: image.Pt(br.Min.X, r.Min.Y-d(60)), title: u.t("adminHint")}
						pointer.CursorPointer.Add(gtx.Ops)
					}
				}
				return layout.Dimensions{Size: sz}
			})
		})
		x += w + d(18)
	}
	if st.OnlineFiles > 0 {
		key := "stOnlineCounted"
		if t.SkipOnline() {
			key = "stOnlineSkipped"
		}
		is := d(14)
		cy := r.Min.Y + r.Dy()/2
		u.icon(gtx, ic.cloud, image.Rect(x, cy-is/2, x+is, cy+is/2), u.pal.Muted)
		x += is + d(5)
		seg(u.t(key, "n", u.num(st.OnlineFiles), "size", u.size(st.OnlineBytes)), nil)
	}
	if t.Running() {
		cur := st.Current
		maxW := r.Max.X - d(12) - x
		if maxW > r.Dx()/2 {
			maxW = r.Dx() / 2
		}
		// show the end of long paths
		for len(cur) > 8 && u.textWidth(gtx, cur, 12, font.Normal) > maxW {
			cur = "…" + cur[len(cur)/5:]
		}
		u.text(gtx, image.Rect(r.Max.X-d(12)-maxW, r.Min.Y, r.Max.X-d(12), r.Max.Y), cur, 12, u.pal.Faint, font.Normal, text.End)
	}
}

// ---------------------------------------------------------------- actions

func (u *UI) startScan(path string) {
	path = strings.TrimSpace(strings.Trim(strings.TrimSpace(path), `"`))
	if path == "" {
		return
	}
	st, err := os.Stat(path)
	if err != nil || !st.IsDir() {
		u.toast(u.t("eNotADir"), true)
		return
	}
	if u.tree != nil {
		u.tree.Cancel()
	}
	t := scanner.NewTree(path, scanner.Options{
		Exclude: u.settings.Exclude, CloudFetch: u.settings.CloudFetch, SkipOnline: u.settings.OnlineFiles != "count",
	})
	rec := []string{t.RootPath}
	for _, r := range u.settings.Recent {
		if !strings.EqualFold(r, t.RootPath) && len(rec) < 8 {
			rec = append(rec, r)
		}
	}
	u.settings.Recent = rec
	_ = saveSettings(u.settings)
	u.tree = t
	u.pathEd.SetText(t.RootPath)
	u.expanded = map[string]bool{"d0": true}
	u.selected, u.focusDir, u.topFilter = "d0", "d0", ""
	u.treeList.off = 0
	u.flat = nil
	u.side.reset()
	u.mapView.data = nil
	u.mapView.dirty = true
	t.Start(context.Background())
	u.rebuildFlat()
	u.requestSide()
}

func (u *UI) rescan(key string) {
	t := u.tree
	if t == nil || key == "" {
		return
	}
	if t.Running() {
		u.toast(u.t("eBusy"), true)
		return
	}
	d, idx, ok := t.Resolve(key)
	if !ok || idx >= 0 {
		return
	}
	t.Rescan(d)
	u.flatDirty = true
	u.requestSide()
	u.win.Invalidate()
}

func (u *UI) dirKeyOf(key string) string {
	if strings.HasPrefix(key, "f") {
		id, _, _ := strings.Cut(key[1:], ":")
		return "d" + id
	}
	return key
}

func (u *UI) pathOf(key string) string {
	if u.tree == nil {
		return ""
	}
	p, _ := u.tree.KeyPath(key)
	return p
}

func (u *UI) copyPath(key string) {
	if p := u.pathOf(key); p != "" {
		u.clip = p
		u.toast(u.t("copied"), false)
	}
}

func (u *UI) openItem(key string) {
	if p := u.pathOf(key); p != "" {
		if err := openItem(p); err != nil {
			u.toast(u.t("eGeneric", "msg", err.Error()), true)
		}
	}
}

func (u *UI) showInExplorer(key string) {
	if p := u.pathOf(key); p != "" {
		if err := showInExplorer(p, strings.HasPrefix(key, "d")); err != nil {
			u.toast(u.t("eGeneric", "msg", err.Error()), true)
		}
	}
}

func (u *UI) askDelete(key string) {
	t := u.tree
	if t == nil || key == "" || key == "d0" {
		return
	}
	if t.Running() {
		u.toast(u.t("eBusy"), true)
		return
	}
	d, idx, ok := t.Resolve(key)
	if !ok {
		return
	}
	p := u.pathOf(key)
	name, size := filepath.Base(p), int64(0)
	if idx >= 0 {
		size = d.Files()[idx].Size
	} else {
		size = d.Size()
	}
	do := func() {
		if err := moveToRecycleBin(p); err != nil {
			u.toast(u.t("eGeneric", "msg", err.Error()), true)
			return
		}
		// resolve again: the tree may have changed while the dialog was open
		if d2, idx2, ok := t.Resolve(key); ok {
			if idx2 >= 0 {
				t.RemoveFile(d2, idx2)
			} else {
				t.RemoveDir(d2)
			}
		}
		if u.selected == key {
			u.selected = u.dirKeyOf(key)
			if idx < 0 && d.Parent != nil {
				u.selected = scanner.DirRow(d.Parent).Key
			}
		}
		u.toast(u.t("deleted", "name", name), false)
		u.flatDirty = true
		u.requestSide()
	}
	if !u.settings.ConfirmDelete {
		do()
		return
	}
	msg := u.t("confirmText", "name", name, "size", u.size(size)) + "\n\n" + p
	u.goSafe(func() {
		if confirm(u.hwnd, u.t("confirmTitle"), msg) {
			u.post(do)
		}
	})
}

func (u *UI) pickFolder() {
	u.goSafe(func() {
		if p := pickFolderDialog(u.hwnd); p != "" {
			u.post(func() { u.startScan(p) })
		}
	})
}

func (u *UI) export() {
	t := u.tree
	if t == nil || t.Running() {
		return
	}
	key := u.focusDir
	if key == "" {
		key = "d0"
	}
	d, _, ok := t.Resolve(key)
	if !ok {
		return
	}
	s := u.settings
	name := strings.Trim(strings.NewReplacer(`\`, "_", ":", "", "/", "_").Replace(d.Name), "_")
	def := "SizeScope_" + name + "_" + time.Now().Format("2006-01-02") + ".csv"
	u.goSafe(func() {
		path := saveFileDialog(u.hwnd, def)
		if path == "" {
			return
		}
		err := writeCSV(t, d, path, s)
		u.post(func() {
			if err != nil {
				u.toast(u.t("eGeneric", "msg", err.Error()), true)
			} else {
				u.toast(u.t("exported", "path", path), false)
			}
		})
	})
}

func writeCSV(t *scanner.Tree, d *scanner.Dir, path string, s Settings) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	f.WriteString("\xEF\xBB\xBF") // BOM so Excel detects UTF-8
	sep := ';'
	switch s.CsvSeparator {
	case ",":
		sep = ','
	case "tab":
		sep = '\t'
	}
	return t.ExportCSV(f, d, s.ExportDepth, s.ExportFiles, sep)
}

func (u *UI) restartAdmin() {
	var args []string
	if u.tree != nil {
		args = append(args, u.tree.RootPath)
	}
	if err := restartAsAdmin(args); err != nil {
		u.toast(u.t("eGeneric", "msg", err.Error()), true)
		return
	}
	os.Exit(0)
}

// itemMenu builds the context menu for a tree row, treemap tile or file list entry.
func (u *UI) itemMenu(at image.Point, key string, origin string) {
	if key == "" || u.tree == nil {
		return
	}
	isDir := strings.HasPrefix(key, "d")
	ctrl := "Ctrl+C"
	del := "Del"
	if u.lang == "de" {
		ctrl, del = "Strg+C", "Entf"
	}
	var items []menuItem
	if !isDir {
		items = append(items, menuItem{icon: ic.open, label: u.t("mOpen"), hint: "Enter", action: func() { u.openItem(key) }})
	}
	items = append(items,
		menuItem{icon: ic.pick, label: u.t("mExplorer"), action: func() { u.showInExplorer(key) }},
		menuItem{icon: ic.copy, label: u.t("mCopyPath"), hint: ctrl, action: func() { u.copyPath(key) }},
	)
	if origin != "tree" {
		items = append(items, menuItem{icon: ic.target, label: u.t("mReveal"), action: func() { u.reveal(key, true) }})
	}
	if isDir {
		items = append(items, menuItem{sep: true})
		if origin == "tree" {
			items = append(items, menuItem{icon: ic.mapIc, label: u.t("mMapRoot"), action: func() { u.setFocus(key); u.tab = 0; u.requestSide() }})
		}
		items = append(items,
			menuItem{icon: ic.refresh, label: u.t("mRescan"), hint: "F5", action: func() { u.rescan(key) }},
			menuItem{icon: ic.play, label: u.t("mScanHere"), action: func() { u.startScan(u.pathOf(key)) }},
		)
	}
	if key != "d0" {
		items = append(items, menuItem{sep: true}, menuItem{icon: ic.trash, label: u.t("mDelete"), hint: del, danger: true, action: func() { u.askDelete(key) }})
	}
	u.openMenu(at, items)
}

func driveIcon(kind string) *widget.Icon {
	switch kind {
	case "network":
		return ic.share
	case "removable":
		return ic.usb
	}
	return ic.drive
}

// showAdminBanner reports whether to warn about missing administrator rights.
func (u *UI) showAdminBanner() bool {
	if u.bannerClosed || !u.settings.AdminHint {
		return false
	}
	if os.Getenv("SIZESCOPE_ADMIN_BANNER") == "1" { // for testing on other systems
		return true
	}
	return !u.admin && platformName == "windows"
}

func (u *UI) adminBanner(gtx layout.Context, r image.Rectangle) {
	d := func(v float32) int { return u.dp(gtx, v) }
	fill(gtx, r, u.pal.WarnSoft)
	fill(gtx, image.Rect(r.Min.X, r.Max.Y-1, r.Max.X, r.Max.Y), u.pal.Line)
	is := d(18)
	cy := r.Min.Y + r.Dy()/2
	x := r.Min.X + d(14)
	u.icon(gtx, ic.shield, image.Rect(x, cy-is/2, x+is, cy+is/2), u.pal.Warn)
	x += is + d(10)
	closeW := d(30)
	label := u.t("restartAdmin")
	bw := u.textWidth(gtx, label, 13, font.SemiBold) + d(28)
	bx := r.Max.X - d(10) - closeW - d(8) - bw
	u.text(gtx, image.Rect(x, r.Min.Y, bx-d(12), r.Max.Y), u.t("adminBanner"), 13, u.pal.Ink, font.Normal, text.Start)
	if u.button(gtx, &u.btnBannerAdmin, image.Rect(bx, cy-d(14), bx+bw, cy+d(14)), label, ic.shield, btnPlain) {
		u.restartAdmin()
	}
	cx := r.Max.X - d(10) - closeW
	if u.iconButton(gtx, &u.btnBannerClose, image.Rect(cx, cy-closeW/2, cx+closeW, cy+closeW/2), ic.close, u.pal.Muted, true) {
		u.bannerClosed = true
	}
}
