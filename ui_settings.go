package main

import (
	"image"
	"strconv"
	"strings"

	"gioui.org/font"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

type settingsDlg struct {
	s                         Settings
	segs                      map[string][]widget.Clickable
	checks                    map[string]*widget.Clickable
	topEd, depthEd, excludeEd widget.Editor
	btnSave, btnCancel        widget.Clickable
	backdrop                  int8
	list                      VList
	scroll                    int
}

func (u *UI) openSettings() {
	dl := &settingsDlg{s: u.settings, segs: map[string][]widget.Clickable{}, checks: map[string]*widget.Clickable{}}
	dl.s.Exclude = append([]string(nil), u.settings.Exclude...)
	dl.topEd.SingleLine, dl.depthEd.SingleLine = true, true
	dl.topEd.Filter, dl.depthEd.Filter = "0123456789", "0123456789"
	dl.topEd.SetText(strconv.Itoa(dl.s.TopFiles))
	dl.depthEd.SetText(strconv.Itoa(dl.s.ExportDepth))
	dl.excludeEd.SetText(strings.Join(dl.s.Exclude, "\n"))
	u.dlg = dl
}

func (dl *settingsDlg) seg(name string, n int) []widget.Clickable {
	if len(dl.segs[name]) != n {
		dl.segs[name] = make([]widget.Clickable, n)
	}
	return dl.segs[name]
}

func (dl *settingsDlg) check(name string) *widget.Clickable {
	if dl.checks[name] == nil {
		dl.checks[name] = new(widget.Clickable)
	}
	return dl.checks[name]
}

func (u *UI) layoutSettings(gtx layout.Context) {
	dl := u.dlg
	d := func(v float32) int { return u.dp(gtx, v) }
	full := image.Rectangle{Max: gtx.Constraints.Max}
	fill(gtx, full, u.pal.Backdrop)
	for range pointerEvents(gtx, &dl.backdrop, pointer.Press) {
	}
	hitArea(gtx, full, &dl.backdrop, pointer.CursorDefault)

	w := d(640)
	if w > full.Dx()-d(40) {
		w = full.Dx() - d(40)
	}
	h := full.Dy() - d(60)
	if h > d(820) {
		h = d(820)
	}
	box := image.Rect((full.Dx()-w)/2, (full.Dy()-h)/2, (full.Dx()+w)/2, (full.Dy()+h)/2)
	u.shadowBox(gtx, box, d(12))
	pad := d(22)
	u.text(gtx, image.Rect(box.Min.X+pad, box.Min.Y+d(16), box.Max.X-pad, box.Min.Y+d(44)), u.t("settings"), 17, u.pal.Ink, font.SemiBold, text.Start)

	// each row: label on the left, control on the right, optional hint below the control
	type row struct {
		label string
		base  float32 // control height in dp
		hint  string
		draw  func(gtx layout.Context, r image.Rectangle)
	}
	segRow := func(label, name string, values, labels []string, sel *string, hint string) row {
		return row{label, 40, hint, func(gtx layout.Context, r image.Rectangle) {
			u.segmented(gtx, dl.seg(name, len(values)), image.Rect(r.Min.X, r.Min.Y+d(4), r.Max.X, r.Max.Y-d(4)), values, labels, sel)
		}}
	}
	checkRow := func(label, name string, v *bool, hint string) row {
		return row{label, 36, hint, func(gtx layout.Context, r image.Rectangle) {
			u.checkbox(gtx, dl.check(name), image.Rect(r.Min.X, r.Min.Y, r.Min.X+d(30), r.Max.Y), v)
		}}
	}
	editRow := func(label string, ed *widget.Editor, height float32, multi bool, hint string) row {
		return row{label, height, hint, func(gtx layout.Context, r image.Rectangle) {
			er := image.Rect(r.Min.X, r.Min.Y+d(4), r.Max.X, r.Max.Y-d(4))
			fillRR(gtx, er, d(6), u.pal.Surface)
			bc := u.pal.Line
			if gtx.Focused(ed) {
				bc = u.pal.Accent
			}
			strokeRR(gtx, er, d(6), 1, bc)
			within(gtx, er.Inset(d(8)), func(gtx layout.Context) {
				gtx.Constraints.Min = image.Point{}
				m := op.Record(gtx.Ops)
				e := material.Editor(u.th, ed, "")
				e.Color = u.pal.Ink
				e.TextSize = 13
				dims := e.Layout(gtx)
				call := m.Stop()
				y := 0
				if !multi {
					y = (gtx.Constraints.Max.Y - dims.Size.Y) / 2
				}
				o := op.Offset(image.Pt(0, y)).Push(gtx.Ops)
				call.Add(gtx.Ops)
				o.Pop()
			})
		}}
	}
	s := &dl.s
	rows := []row{
		segRow(u.t("sLanguage"), "lang", []string{"auto", "de", "en"}, []string{u.t("sAuto"), "Deutsch", "English"}, &s.Language, ""),
		segRow(u.t("sTheme"), "theme", []string{"system", "light", "dark"}, []string{u.t("sSystem"), u.t("sLight"), u.t("sDark")}, &s.Theme, ""),
		segRow(u.t("sUnit"), "unit", []string{"auto", "KB", "MB", "GB"}, []string{u.t("sAuto"), "KB", "MB", "GB"}, &s.Unit, ""),
		segRow(u.t("sSizeMode"), "mode", []string{"size", "alloc"}, []string{u.t("colSize"), u.t("colAlloc")}, &s.SizeMode, ""),
		segRow(u.t("sMapColor"), "map", []string{"type", "depth"}, []string{u.t("sByType"), u.t("sByFolder")}, &s.MapColor, ""),
		checkRow(u.t("sShowFiles"), "files", &s.ShowFiles, ""),
		editRow(u.t("sTopFiles"), &dl.topEd, 38, false, ""),
		editRow(u.t("sExclude"), &dl.excludeEd, 78, true, u.t("sExcludeHint")),
		segRow(u.t("sOnline"), "online", []string{"skip", "count"}, []string{u.t("sOnlineSkip"), u.t("sOnlineCount")}, &s.OnlineFiles, u.t("sOnlineHint")),
		checkRow(u.t("sCloudFetch"), "cloud", &s.CloudFetch, u.t("sCloudFetchHint")),
		segRow(u.t("sCsvSep"), "csv", []string{";", ",", "tab"}, []string{";", ",", "Tab"}, &s.CsvSeparator, ""),
		editRow(u.t("sExportDepth"), &dl.depthEd, 38, false, u.t("sExportDepthHint")),
		checkRow(u.t("sExportFiles"), "expfiles", &s.ExportFiles, ""),
		checkRow(u.t("sConfirmDelete"), "confirm", &s.ConfirmDelete, ""),
		checkRow(u.t("sAdminHint"), "adminhint", &s.AdminHint, ""),
		checkRow(u.t("sBackupRead"), "backup", &s.BackupRead, u.t("sBackupReadHint")),
	}

	content := image.Rect(box.Min.X+pad, box.Min.Y+d(56), box.Max.X-pad, box.Max.Y-d(64))
	labelW := d(210)
	ctrlW := content.Dx() - labelW - d(8) // leave room for the scroll indicator
	gap := d(8)
	// measure first: labels and hints wrap differently depending on font and DPI
	measure := func(w int, txt string, size float32) int {
		if txt == "" {
			return 0
		}
		m := op.Record(gtx.Ops)
		h := u.wrapText(gtx, image.Point{}, w, txt, unit.Sp(size), u.pal.Muted)
		m.Stop()
		return h
	}
	heights := make([]int, len(rows))
	hintH := make([]int, len(rows))
	total := 0
	for i, r := range rows {
		hintH[i] = measure(ctrlW, r.hint, 11)
		h := d(r.base)
		if hintH[i] > 0 {
			h += hintH[i] + d(4)
		}
		if lh := measure(labelW-d(16), r.label, 13) + d(18); lh > h {
			h = lh
		}
		heights[i] = h
		total += h + gap
	}
	about := u.t("about", "v", appVersion)
	total += measure(content.Dx(), about, 12) + d(12)

	// scroll with the mouse wheel if the dialog is too small
	for _, e := range pointerEvents(gtx, &dl.list.tag, pointer.Scroll) {
		dl.scroll += int(e.Scroll.Y)
	}
	maxScroll := max(total-content.Dy(), 0)
	dl.scroll = min(max(dl.scroll, 0), maxScroll)
	within(gtx, content, func(gtx layout.Context) {
		// registered first so it lies below the controls and only catches the wheel
		hitArea(gtx, image.Rectangle{Max: content.Size()}, &dl.list.tag, pointer.CursorDefault)
		y := -dl.scroll
		for i, r := range rows {
			u.wrapText(gtx, image.Pt(0, y+d(9)), labelW-d(16), r.label, 13, u.pal.Muted)
			r.draw(gtx, image.Rect(labelW, y, labelW+ctrlW, y+d(r.base)))
			if r.hint != "" {
				u.wrapText(gtx, image.Pt(labelW, y+d(r.base)+d(2)), ctrlW, r.hint, 11, u.pal.Faint)
			}
			y += heights[i] + gap
		}
		u.wrapText(gtx, image.Pt(0, y+d(4)), content.Dx(), about, 12, u.pal.Faint)
		if maxScroll > 0 {
			// thin scroll indicator so it is clear there is more below
			ch := content.Dy()
			th := ch * ch / total
			ty := (ch - th) * dl.scroll / maxScroll
			fillRR(gtx, image.Rect(content.Dx()-d(4), ty, content.Dx(), ty+th), d(2), u.pal.Line)
		}
	})

	by := box.Max.Y - d(52)
	bw := d(130)
	if u.button(gtx, &dl.btnSave, image.Rect(box.Max.X-pad-bw, by, box.Max.X-pad, by+d(34)), u.t("save"), nil, btnPrimary) {
		n, _ := strconv.Atoi(dl.topEd.Text())
		if n <= 0 {
			n = 200
		}
		s.TopFiles = n
		s.ExportDepth, _ = strconv.Atoi(dl.depthEd.Text())
		s.Exclude = nil
		for _, l := range strings.Split(dl.excludeEd.Text(), "\n") {
			if l = strings.TrimSpace(l); l != "" {
				s.Exclude = append(s.Exclude, l)
			}
		}
		u.settings = *s
		if err := saveSettings(u.settings); err != nil {
			u.toast(u.t("eGeneric", "msg", err.Error()), true)
		}
		u.applySettings()
		u.dlg = nil
		u.requestSide()
		return
	}
	if u.button(gtx, &dl.btnCancel, image.Rect(box.Max.X-pad-2*bw-d(8), by, box.Max.X-pad-bw-d(8), by+d(34)), u.t("cancel"), nil, btnPlain) {
		u.dlg = nil
	}
}
