package main

import (
	"context"
	"image"
	"image/color"
	"os"
	"strings"
	"sync"
	"time"

	"gioui.org/font"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/widget"

	"sizescope/cleanup"
)

const (
	phaseSelect = iota
	phaseRunning
	phaseResults
)

type cleanDlg struct {
	defs    []*cleanup.Definition
	sel     map[string]bool
	checks  map[string]*widget.Clickable
	removes map[string]*widget.Clickable
	phase   int
	dry     bool

	mu       sync.Mutex
	progress cleanup.Progress
	results  []cleanup.Result
	cancel   context.CancelFunc
	started  time.Time

	selList, resList                                                       VList
	backdrop                                                               int8
	btnImport, btnFolder, btnPreview, btnRun, btnClose, btnBack, btnRescan widget.Clickable
	btnCancel, btnRunNow                                                   widget.Clickable
}

func (u *UI) openCleanup() {
	dl := &cleanDlg{sel: map[string]bool{}, checks: map[string]*widget.Clickable{}, removes: map[string]*widget.Clickable{}}
	dl.selList.Hover, dl.resList.Hover = -1, -1
	u.cleanup = dl
	u.reloadCleanups(true)
}

func (u *UI) reloadCleanups(initial bool) {
	dl := u.cleanup
	defs := cleanup.Builtin()
	imported, errs := cleanup.LoadDir(cleanupDir())
	defs = append(defs, imported...)
	for _, e := range errs {
		u.toast(u.t("cImportError", "msg", e.Error()), true)
	}
	dl.defs = defs
	for _, d := range defs {
		if _, ok := dl.sel[d.ID]; !ok && initial {
			dl.sel[d.ID] = d.Default && (!d.Admin || u.admin)
		}
		if dl.checks[d.ID] == nil {
			dl.checks[d.ID] = new(widget.Clickable)
			dl.removes[d.ID] = new(widget.Clickable)
		}
	}
}

func (u *UI) selectedCleanups() []*cleanup.Definition {
	var out []*cleanup.Definition
	for _, d := range u.cleanup.defs {
		if u.cleanup.sel[d.ID] && (!d.Admin || u.admin) {
			out = append(out, d)
		}
	}
	return out
}

func (u *UI) startCleanup(dry bool) {
	dl := u.cleanup
	defs := u.selectedCleanups()
	if len(defs) == 0 {
		return
	}
	run := func() {
		ctx, cancel := context.WithCancel(context.Background())
		dl.mu.Lock()
		dl.cancel, dl.results, dl.progress, dl.started = cancel, nil, cleanup.Progress{}, time.Now()
		dl.mu.Unlock()
		dl.phase, dl.dry = phaseRunning, dry
		dl.resList.off = 0
		u.goSafe(func() {
			res := cleanup.Run(ctx, defs, dry, func(p cleanup.Progress) {
				dl.mu.Lock()
				dl.progress = p
				dl.mu.Unlock()
				u.win.Invalidate()
			})
			u.post(func() {
				dl.mu.Lock()
				dl.results = res
				dl.mu.Unlock()
				dl.phase = phaseResults
				if !dry && u.tree != nil {
					u.toast(u.t("cRescanHint"), false)
				}
			})
		})
	}
	if dry {
		run()
		return
	}
	var names []string
	for _, d := range defs {
		names = append(names, "• "+d.Name.Get(u.lang))
	}
	msg := u.t("cConfirmText") + "\n\n" + strings.Join(names, "\n")
	u.goSafe(func() {
		if confirm(u.hwnd, u.t("cConfirmTitle"), msg) {
			u.post(run)
		}
	})
}

func (u *UI) importCleanup() {
	u.goSafe(func() {
		p := openFileDialog(u.hwnd, u.t("cFilter"))
		if p == "" {
			return
		}
		_, err := cleanup.Import(p, cleanupDir())
		u.post(func() {
			if err != nil {
				u.toast(u.t("cImportError", "msg", err.Error()), true)
				return
			}
			u.toast(u.t("cImported"), false)
			if u.cleanup != nil {
				u.reloadCleanups(false)
			}
		})
	})
}

// ---------------------------------------------------------------- layout

func (u *UI) layoutCleanup(gtx layout.Context) {
	dl := u.cleanup
	d := func(v float32) int { return u.dp(gtx, v) }
	full := image.Rectangle{Max: gtx.Constraints.Max}
	fill(gtx, full, u.pal.Backdrop)
	for range pointerEvents(gtx, &dl.backdrop, pointer.Press) {
	}
	hitArea(gtx, full, &dl.backdrop, pointer.CursorDefault)

	w := min(d(820), full.Dx()-d(40))
	h := min(d(760), full.Dy()-d(40))
	box := image.Rect((full.Dx()-w)/2, (full.Dy()-h)/2, (full.Dx()+w)/2, (full.Dy()+h)/2)
	u.shadowBox(gtx, box, d(12))
	pad := d(22)
	u.text(gtx, image.Rect(box.Min.X+pad, box.Min.Y+d(16), box.Max.X-pad, box.Min.Y+d(44)), u.t("cTitle"), 17, u.pal.Ink, font.SemiBold, text.Start)
	content := image.Rect(box.Min.X+pad, box.Min.Y+d(54), box.Max.X-pad, box.Max.Y-d(66))
	foot := image.Rect(box.Min.X+pad, box.Max.Y-d(52), box.Max.X-pad, box.Max.Y-d(18))

	switch dl.phase {
	case phaseSelect:
		u.cleanupSelect(gtx, content, foot)
	case phaseRunning:
		u.cleanupRunning(gtx, content, foot)
	default:
		u.cleanupResults(gtx, content, foot)
	}
}

func (u *UI) cleanupSelect(gtx layout.Context, content, foot image.Rectangle) {
	dl := u.cleanup
	d := func(v float32) int { return u.dp(gtx, v) }
	lead := u.t("cLead")
	lh := u.wrapText(gtx, content.Min, content.Dx(), lead, 13, u.pal.Muted)
	lr := image.Rect(content.Min.X, content.Min.Y+lh+d(10), content.Max.X, content.Max.Y)
	fill(gtx, image.Rect(lr.Min.X, lr.Min.Y-1, lr.Max.X, lr.Min.Y), u.pal.Line)
	rowH := d(60)
	defs := dl.defs
	within(gtx, lr, func(gtx layout.Context) {
		size := lr.Size()
		dl.selList.Update(gtx, len(defs), rowH, size)
		dl.selList.Layout(gtx, u, size, len(defs), rowH, func(gtx layout.Context, i int, rr image.Rectangle) {
			def := defs[i]
			allowed := !def.Admin || u.admin
			fill(gtx, image.Rect(rr.Min.X, rr.Max.Y-1, rr.Max.X, rr.Max.Y), u.pal.LineSoft)
			chk := dl.checks[def.ID]
			if chk.Clicked(gtx) && allowed {
				dl.sel[def.ID] = !dl.sel[def.ID]
			}
			removable := def.File != ""
			rmW := 0
			if removable {
				rmW = d(36)
			}
			within(gtx, image.Rect(rr.Min.X, rr.Min.Y, rr.Max.X-rmW, rr.Max.Y), func(gtx layout.Context) {
				chk.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					sz := gtx.Constraints.Min
					if chk.Hovered() && allowed {
						fill(gtx, image.Rectangle{Max: sz}, u.pal.Hover)
					}
					// checkbox
					s := d(18)
					bx := image.Rect(d(8), d(12), d(8)+s, d(12)+s)
					on := dl.sel[def.ID] && allowed
					switch {
					case on:
						fillRR(gtx, bx, d(4), u.pal.Accent)
						u.icon(gtx, ic.check, bx.Inset(d(2)), u.pal.AccentInk)
					case allowed:
						strokeRR(gtx, bx, d(4), float32(d(1.5)), u.pal.Faint)
					default:
						fillRR(gtx, bx, d(4), u.pal.LineSoft)
					}
					x := d(38)
					col := u.pal.Ink
					if !allowed {
						col = u.pal.Faint
					}
					nw := u.text(gtx, image.Rect(x, d(8), sz.X-d(8), d(30)), def.Name.Get(u.lang), 14, col, font.SemiBold, text.Start)
					// badges
					bxp := x + nw + d(10)
					for _, b := range u.cleanupBadges(def, allowed) {
						bw := u.textWidth(gtx, b.label, 11, font.Normal) + d(12)
						r := image.Rect(bxp, d(11), bxp+bw, d(28))
						fillRR(gtx, r, d(4), b.bg)
						u.text(gtx, r, b.label, 11, b.fg, font.Normal, text.Middle)
						bxp += bw + d(6)
					}
					desc := def.Description.Get(u.lang)
					if desc == "" && def.Script != "" {
						desc = def.File
					}
					u.text(gtx, image.Rect(x, d(32), sz.X-d(8), d(52)), desc, 12, u.pal.Muted, font.Normal, text.Start)
					if chk.Hovered() && u.textWidth(gtx, desc, 12, font.Normal) > sz.X-d(8)-x {
						// full description when it does not fit
						u.tip = &tipInfo{at: lr.Min.Add(rr.Min).Add(image.Pt(x, d(50))), title: def.Name.Get(u.lang), lines: wrapWords(desc, 70)}
					}
					if allowed {
						pointer.CursorPointer.Add(gtx.Ops)
					}
					return layout.Dimensions{Size: sz}
				})
			})
			if removable {
				rb := dl.removes[def.ID]
				r := image.Rect(rr.Max.X-rmW, rr.Min.Y+(rowH-d(30))/2, rr.Max.X-d(6), rr.Min.Y+(rowH+d(30))/2)
				if u.iconButton(gtx, rb, r, ic.trash, u.pal.Muted, true) {
					file := def.File
					name := def.Name.Get(u.lang)
					u.goSafe(func() {
						if confirm(u.hwnd, u.t("cRemoveTitle"), u.t("cRemoveText", "name", name)) {
							u.post(func() {
								if err := os.Remove(file); err != nil {
									u.toast(u.t("eGeneric", "msg", err.Error()), true)
								}
								delete(dl.sel, def.ID)
								u.reloadCleanups(false)
							})
						}
					})
				}
			}
		})
	})

	// footer
	x := foot.Min.X
	bw := u.textWidth(gtx, u.t("cImport"), 13, font.Normal) + d(48)
	if u.button(gtx, &dl.btnImport, image.Rect(x, foot.Min.Y, x+bw, foot.Max.Y), u.t("cImport"), ic.export, btnPlain) {
		u.importCleanup()
	}
	x += bw + d(8)
	if u.iconButton(gtx, &dl.btnFolder, image.Rect(x, foot.Min.Y, x+foot.Dy(), foot.Max.Y), ic.pick, u.pal.Muted, true) {
		dir := cleanupDir()
		_ = os.MkdirAll(dir, 0o755)
		_ = showInExplorer(dir, true)
	}
	n := len(u.selectedCleanups())
	rx := foot.Max.X
	runW := u.textWidth(gtx, u.t("cRun"), 13, font.SemiBold) + d(56)
	if u.button(gtx, &dl.btnRun, image.Rect(rx-runW, foot.Min.Y, rx, foot.Max.Y), u.t("cRun"), ic.sweep, btnPrimary) && n > 0 {
		u.startCleanup(false)
	}
	rx -= runW + d(8)
	pw := u.textWidth(gtx, u.t("cPreview"), 13, font.Normal) + d(36)
	if u.button(gtx, &dl.btnPreview, image.Rect(rx-pw, foot.Min.Y, rx, foot.Max.Y), u.t("cPreview"), nil, btnPlain) && n > 0 {
		u.startCleanup(true)
	}
	rx -= pw + d(8)
	cw := u.textWidth(gtx, u.t("close"), 13, font.Normal) + d(36)
	if u.button(gtx, &dl.btnClose, image.Rect(rx-cw, foot.Min.Y, rx, foot.Max.Y), u.t("close"), nil, btnGhost) {
		u.cleanup = nil
	}
}

type badgeInfo struct {
	label  string
	bg, fg color.NRGBA
}

func (u *UI) cleanupBadges(def *cleanup.Definition, allowed bool) []badgeInfo {
	var out []badgeInfo
	if def.Admin {
		bg, fg := u.pal.WarnSoft, u.pal.Warn
		label := u.t("cAdmin")
		if !allowed {
			label = u.t("cAdminRequired")
		}
		out = append(out, badgeInfo{label, bg, fg})
	}
	if def.Slow {
		out = append(out, badgeInfo{u.t("cSlow"), u.pal.LineSoft, u.pal.Muted})
	}
	if def.Script != "" {
		out = append(out, badgeInfo{"PowerShell", u.pal.AccentSoft, u.pal.Ink})
	} else if def.File != "" {
		out = append(out, badgeInfo{u.t("cImportedBadge"), u.pal.AccentSoft, u.pal.Ink})
	}
	return out
}

func (u *UI) cleanupRunning(gtx layout.Context, content, foot image.Rectangle) {
	dl := u.cleanup
	d := func(v float32) int { return u.dp(gtx, v) }
	gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(250 * time.Millisecond)})
	dl.mu.Lock()
	p := dl.progress
	started := dl.started
	dl.mu.Unlock()
	name := ""
	for _, def := range dl.defs {
		if def.ID == p.Def {
			name = def.Name.Get(u.lang)
		}
	}
	key := "cRunning"
	if dl.dry {
		key = "cPreviewing"
	}
	y := content.Min.Y + d(30)
	u.text(gtx, image.Rect(content.Min.X, y, content.Max.X, y+d(28)), u.t(key)+"  "+dur(time.Since(started)), 16, u.pal.Ink, font.SemiBold, text.Start)
	y += d(36)
	// indeterminate bar
	bar := image.Rect(content.Min.X, y, content.Max.X, y+d(6))
	fillRR(gtx, bar, d(3), u.pal.BarTrack)
	t := float64(time.Since(started).Milliseconds()%1600) / 1600
	seg := bar.Dx() / 4
	sx := bar.Min.X + int(float64(bar.Dx()+seg)*t) - seg
	within(gtx, bar, func(gtx layout.Context) {
		fillRR(gtx, image.Rect(sx-bar.Min.X, 0, sx-bar.Min.X+seg, bar.Dy()), d(3), u.pal.Accent)
	})
	y += d(22)
	if name != "" {
		u.text(gtx, image.Rect(content.Min.X, y, content.Max.X, y+d(22)), name, 13, u.pal.Ink, font.Normal, text.Start)
		y += d(22)
	}
	u.text(gtx, image.Rect(content.Min.X, y, content.Max.X, y+d(20)), p.Path, 12, u.pal.Muted, font.Normal, text.Start)
	y += d(28)
	u.text(gtx, image.Rect(content.Min.X, y, content.Max.X, y+d(22)), u.t("cProgress", "n", u.num(p.Files), "size", u.size(p.Bytes)), 13, u.pal.Muted, font.Normal, text.Start)

	bw := u.textWidth(gtx, u.t("cancel"), 13, font.Normal) + d(36)
	if u.button(gtx, &dl.btnCancel, image.Rect(foot.Max.X-bw, foot.Min.Y, foot.Max.X, foot.Max.Y), u.t("cancel"), nil, btnPlain) {
		dl.mu.Lock()
		if dl.cancel != nil {
			dl.cancel()
		}
		dl.mu.Unlock()
	}
}

// result list rows: a header per cleanup followed by its locations
type resRow struct {
	header bool
	res    *cleanup.Result
	loc    *cleanup.Location
}

func (u *UI) cleanupResults(gtx layout.Context, content, foot image.Rectangle) {
	dl := u.cleanup
	d := func(v float32) int { return u.dp(gtx, v) }
	dl.mu.Lock()
	results := dl.results
	dl.mu.Unlock()
	var total, files, skipped int64
	var rows []resRow
	for i := range results {
		r := &results[i]
		total += r.Bytes
		files += r.Files
		skipped += r.Skipped
		rows = append(rows, resRow{header: true, res: r})
		for j := range r.Locations {
			rows = append(rows, resRow{res: r, loc: &r.Locations[j]})
		}
	}
	key := "cFreed"
	if dl.dry {
		key = "cWouldFree"
	}
	y := content.Min.Y
	u.text(gtx, image.Rect(content.Min.X, y, content.Max.X, y+d(34)), u.t(key, "size", u.size(total)), 22, u.pal.Ink, font.SemiBold, text.Start)
	y += d(36)
	sum := u.t("cSummary", "n", u.num(files))
	if skipped > 0 {
		sum += "  ·  " + u.t("cSkipped", "n", u.num(skipped))
	}
	u.text(gtx, image.Rect(content.Min.X, y, content.Max.X, y+d(20)), sum, 13, u.pal.Muted, font.Normal, text.Start)
	y += d(30)

	// table header
	sizeW, filesW, skipW := d(96), d(84), d(110)
	cols := func(r image.Rectangle) (name, fl, sz, sk image.Rectangle) {
		x3 := r.Max.X - skipW
		x2 := x3 - sizeW
		x1 := x2 - filesW
		return image.Rect(r.Min.X+d(6), r.Min.Y, x1-d(8), r.Max.Y), image.Rect(x1, r.Min.Y, x2-d(6), r.Max.Y),
			image.Rect(x2, r.Min.Y, x3-d(6), r.Max.Y), image.Rect(x3, r.Min.Y, r.Max.X-d(6), r.Max.Y)
	}
	hr := image.Rect(content.Min.X, y, content.Max.X-d(10), y+d(26))
	hn, hf, hs, hk := cols(hr)
	u.text(gtx, hn, u.t("cWhere"), 12, u.pal.Muted, font.Normal, text.Start)
	u.text(gtx, hf, u.t("colFiles"), 12, u.pal.Muted, font.Normal, text.End)
	u.text(gtx, hs, u.t("colSize"), 12, u.pal.Muted, font.Normal, text.End)
	u.text(gtx, hk, u.t("cSkippedCol"), 12, u.pal.Muted, font.Normal, text.End)
	fill(gtx, image.Rect(content.Min.X, hr.Max.Y-1, content.Max.X, hr.Max.Y), u.pal.Line)
	lr := image.Rect(content.Min.X, hr.Max.Y, content.Max.X, content.Max.Y)
	rowH := d(28)
	within(gtx, lr, func(gtx layout.Context) {
		size := lr.Size()
		dl.resList.Update(gtx, len(rows), rowH, size)
		dl.resList.Layout(gtx, u, size, len(rows), rowH, func(gtx layout.Context, i int, rr image.Rectangle) {
			row := rows[i]
			n, f, s, k := cols(rr)
			if row.header {
				fill(gtx, rr, u.pal.Bg)
				label := row.res.Def.Name.Get(u.lang)
				if len(row.res.Locations) == 0 && row.res.Err == "" {
					label += "  –  " + u.t("cNothing")
				}
				if row.res.Err != "" {
					label += "  –  " + u.noteText(row.res.Err)
				}
				u.text(gtx, n, label, 13, u.pal.Ink, font.SemiBold, text.Start)
				u.text(gtx, f, u.num(row.res.Files), 13, u.pal.Ink, font.SemiBold, text.End)
				sz := u.size(row.res.Bytes)
				if row.res.Estimated {
					sz = "≈ " + sz
				}
				u.text(gtx, s, sz, 13, u.pal.Ink, font.SemiBold, text.End)
				if row.res.Skipped > 0 {
					u.text(gtx, k, u.num(row.res.Skipped), 13, u.pal.Warn, font.SemiBold, text.End)
				}
				return
			}
			fill(gtx, image.Rect(rr.Min.X, rr.Max.Y-1, rr.Max.X, rr.Max.Y), u.pal.LineSoft)
			l := row.loc
			path := l.Path
			if path == "recyclebin" {
				path = u.t("cRecycleBin")
			}
			n.Min.X += d(14)
			pw := u.text(gtx, n, path, 12, u.pal.Ink, font.Normal, text.Start)
			if l.Note != "" {
				nr := image.Rect(n.Min.X+pw+d(10), n.Min.Y, n.Max.X, n.Max.Y)
				u.text(gtx, nr, u.noteText(l.Note), 11, u.pal.Faint, font.Normal, text.Start)
			}
			if l.Note != "nopreview" {
				u.text(gtx, f, u.num(l.Files), 12, u.pal.Muted, font.Normal, text.End)
				u.text(gtx, s, u.size(l.Bytes), 12, u.pal.Ink, font.Normal, text.End)
			}
			if l.Skipped > 0 {
				sk := u.num(l.Skipped)
				if l.SkippedBytes > 0 {
					sk += " (" + u.size(l.SkippedBytes) + ")"
				}
				u.text(gtx, k, sk, 12, u.pal.Warn, font.Normal, text.End)
			}
		})
	})

	// footer
	bw := u.textWidth(gtx, u.t("cBack"), 13, font.Normal) + d(36)
	if u.button(gtx, &dl.btnBack, image.Rect(foot.Min.X, foot.Min.Y, foot.Min.X+bw, foot.Max.Y), u.t("cBack"), nil, btnPlain) {
		dl.phase = phaseSelect
	}
	rx := foot.Max.X
	cw := u.textWidth(gtx, u.t("close"), 13, font.Normal) + d(36)
	if u.button(gtx, &dl.btnClose, image.Rect(rx-cw, foot.Min.Y, rx, foot.Max.Y), u.t("close"), nil, btnPlain) {
		u.cleanup = nil
		return
	}
	rx -= cw + d(8)
	if dl.dry {
		rw := u.textWidth(gtx, u.t("cRun"), 13, font.SemiBold) + d(56)
		if u.button(gtx, &dl.btnRunNow, image.Rect(rx-rw, foot.Min.Y, rx, foot.Max.Y), u.t("cRun"), ic.sweep, btnPrimary) {
			u.startCleanup(false)
		}
	} else if u.tree != nil && !u.tree.Running() {
		rw := u.textWidth(gtx, u.t("cRescan"), 13, font.Normal) + d(56)
		if u.button(gtx, &dl.btnRescan, image.Rect(rx-rw, foot.Min.Y, rx, foot.Max.Y), u.t("cRescan"), ic.refresh, btnPlain) {
			p := u.tree.RootPath
			u.cleanup = nil
			u.startScan(p)
		}
	}
}

func (u *UI) noteText(n string) string {
	switch {
	case n == "nopreview":
		return u.t("cNoPreview")
	case n == "canceled":
		return u.t("cCanceled")
	case strings.HasPrefix(n, "refused"):
		return u.t("cRefused")
	}
	return n
}

// wrapWords splits s into lines of at most n characters for tooltips.
func wrapWords(s string, n int) []string {
	var lines []string
	cur := ""
	for _, w := range strings.Fields(s) {
		if cur != "" && len([]rune(cur))+1+len([]rune(w)) > n {
			lines = append(lines, cur)
			cur = w
			continue
		}
		if cur != "" {
			cur += " "
		}
		cur += w
	}
	if cur != "" {
		lines = append(lines, cur)
	}
	return lines
}
