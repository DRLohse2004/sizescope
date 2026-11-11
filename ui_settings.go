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

	type row struct {
		label string
		h     float32
		draw  func(gtx layout.Context, r image.Rectangle)
	}
	segRow := func(name string, values, labels []string, sel *string) row {
		return row{"", 40, func(gtx layout.Context, r image.Rectangle) {
			u.segmented(gtx, dl.seg(name, len(values)), image.Rect(r.Min.X, r.Min.Y+d(4), r.Max.X, r.Max.Y-d(4)), values, labels, sel)
		}}
	}
	checkRow := func(name string, v *bool, hint string) row {
		hh := float32(36)
		if hint != "" {
			hh = 56
		}
		return row{"", hh, func(gtx layout.Context, r image.Rectangle) {
			u.checkbox(gtx, dl.check(name), image.Rect(r.Min.X, r.Min.Y, r.Min.X+d(30), r.Min.Y+d(36)), v)
			if hint != "" {
				u.wrapText(gtx, image.Pt(r.Min.X, r.Min.Y+d(32)), r.Dx(), hint, 11, u.pal.Faint)
			}
		}}
	}
	editRow := func(ed *widget.Editor, hint string, height float32, multi bool) row {
		hh := height
		if hint != "" {
			hh += 18
		}
		return row{"", hh, func(gtx layout.Context, r image.Rectangle) {
			er := image.Rect(r.Min.X, r.Min.Y+d(4), r.Max.X, r.Min.Y+d(height)-d(4))
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
			if hint != "" {
				u.text(gtx, image.Rect(r.Min.X, er.Max.Y+d(2), r.Max.X, er.Max.Y+d(18)), hint, 11, u.pal.Faint, font.Normal, text.Start)
			}
		}}
	}
	s := &dl.s
	rows := []struct {
		label string
		r     row
	}{
		{u.t("sLanguage"), segRow("lang", []string{"auto", "de", "en"}, []string{u.t("sAuto"), "Deutsch", "English"}, &s.Language)},
		{u.t("sTheme"), segRow("theme", []string{"system", "light", "dark"}, []string{u.t("sSystem"), u.t("sLight"), u.t("sDark")}, &s.Theme)},
		{u.t("sUnit"), segRow("unit", []string{"auto", "KB", "MB", "GB"}, []string{u.t("sAuto"), "KB", "MB", "GB"}, &s.Unit)},
		{u.t("sSizeMode"), segRow("mode", []string{"size", "alloc"}, []string{u.t("colSize"), u.t("colAlloc")}, &s.SizeMode)},
		{u.t("sMapColor"), segRow("map", []string{"type", "depth"}, []string{u.t("sByType"), u.t("sByFolder")}, &s.MapColor)},
		{u.t("sShowFiles"), checkRow("files", &s.ShowFiles, "")},
		{u.t("sTopFiles"), editRow(&dl.topEd, "", 38, false)},
		{u.t("sExclude"), editRow(&dl.excludeEd, u.t("sExcludeHint"), 78, true)},
		{u.t("sOnline"), segRow("online", []string{"skip", "count"}, []string{u.t("sOnlineSkip"), u.t("sOnlineCount")}, &s.OnlineFiles)},
		{"", row{"", 30, func(gtx layout.Context, r image.Rectangle) {
			u.wrapText(gtx, image.Pt(r.Min.X, r.Min.Y-d(4)), r.Dx(), u.t("sOnlineHint"), 11, u.pal.Faint)
		}}},
		{u.t("sCloudFetch"), checkRow("cloud", &s.CloudFetch, u.t("sCloudFetchHint"))},
		{u.t("sCsvSep"), segRow("csv", []string{";", ",", "tab"}, []string{";", ",", "Tab"}, &s.CsvSeparator)},
		{u.t("sExportDepth"), editRow(&dl.depthEd, u.t("sExportDepthHint"), 38, false)},
		{u.t("sExportFiles"), checkRow("expfiles", &s.ExportFiles, "")},
		{u.t("sConfirmDelete"), checkRow("confirm", &s.ConfirmDelete, "")},
	}

	content := image.Rect(box.Min.X+pad, box.Min.Y+d(56), box.Max.X-pad, box.Max.Y-d(64))
	labelW := d(200)
	total := 0
	for _, r := range rows {
		total += d(r.r.h) + d(6)
	}
	total += d(30)
	// scroll with the mouse wheel if the dialog is too small
	for _, e := range pointerEvents(gtx, &dl.list.tag, pointer.Scroll) {
		dl.scroll += int(e.Scroll.Y)
	}
	maxScroll := total - content.Dy()
	if maxScroll < 0 {
		maxScroll = 0
	}
	if dl.scroll > maxScroll {
		dl.scroll = maxScroll
	}
	if dl.scroll < 0 {
		dl.scroll = 0
	}
	within(gtx, content, func(gtx layout.Context) {
		y := -dl.scroll
		cw := content.Dx()
		for _, r := range rows {
			hh := d(r.r.h)
			if r.label != "" {
				lh := u.wrapText(gtx, image.Pt(0, y+d(9)), labelW-d(16), r.label, 13, u.pal.Muted)
				if lh > d(24) && hh < lh+d(12) {
					hh = lh + d(12) // label wraps onto two lines
				}
			}
			r.r.draw(gtx, image.Rect(labelW, y, cw, y+hh))
			y += hh + d(6)
		}
		u.text(gtx, image.Rect(0, y, cw, y+d(24)), u.t("about", "v", appVersion), 12, u.pal.Faint, font.Normal, text.Start)
		if maxScroll > 0 {
			// thin scroll indicator so it is clear there is more below
			ch := content.Dy()
			th := ch * ch / total
			ty := (ch - th) * dl.scroll / maxScroll
			fillRR(gtx, image.Rect(cw-d(4), ty, cw, ty+th), d(2), u.pal.Line)
		}
		hitArea(gtx, image.Rectangle{Max: content.Size()}, &dl.list.tag, pointer.CursorDefault)
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
