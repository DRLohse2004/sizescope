package main

import (
	"image"
	"image/color"
	"sort"
	"sync"
	"time"

	"gioui.org/font"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/text"
	"gioui.org/widget"

	"sizescope/scanner"
)

func toFilters(fs []key.Filter) []event.Filter {
	out := make([]event.Filter, len(fs))
	for i, f := range fs {
		out[i] = f
	}
	return out
}

// sideState holds results of background queries for the side panel.
type sideState struct {
	mu          sync.Mutex
	seq         int
	key         string
	tab         int
	filter      string
	mapData     *scanner.MapNode
	exts        []scanner.ExtStat
	top         []scanner.FileInfo
	ages        []scanner.AgeBucket
	lastRunning bool
}

func (s *sideState) reset() {
	s.mu.Lock()
	s.seq++
	s.mapData, s.exts, s.top, s.ages = nil, nil, nil, nil
	s.mu.Unlock()
}

// requestSide recomputes the data of the active tab in the background.
func (u *UI) requestSide() {
	t := u.tree
	if t == nil {
		return
	}
	u.sideAt = time.Now()
	key, tab, filter, topN := u.focusDir, u.tab, u.topFilter, u.settings.TopFiles
	if key == "" {
		key = "d0"
	}
	u.side.mu.Lock()
	u.side.seq++
	seq := u.side.seq
	u.side.mu.Unlock()
	u.buildCrumbs(key)
	go func() {
		d, idx, ok := t.Resolve(key)
		if !ok || idx >= 0 {
			d = t.Root
			key = "d0"
		}
		var (
			m    *scanner.MapNode
			exts []scanner.ExtStat
			top  []scanner.FileInfo
			ages []scanner.AgeBucket
		)
		switch tab {
		case 0:
			m = t.Treemap(d, 5000, 0.00005)
		case 1:
			exts = t.Extensions(d, 300)
		case 2:
			if topN <= 0 {
				topN = 200
			}
			top = t.TopFiles(d, topN, filter)
		case 3:
			ages = t.Ages(d, time.Now())
		}
		u.post(func() {
			s := &u.side
			s.mu.Lock()
			defer s.mu.Unlock()
			if seq != s.seq {
				return
			}
			s.key, s.tab, s.filter = key, tab, filter
			switch tab {
			case 0:
				s.mapData = m
				u.mapView.data = m
				u.mapView.dirty = true
			case 1:
				s.exts = exts
			case 2:
				s.top = top
			case 3:
				s.ages = ages
			}
		})
	}()
}

func (u *UI) buildCrumbs(key string) {
	t := u.tree
	d, idx, ok := t.Resolve(key)
	if !ok || idx >= 0 {
		u.crumbs = []string{"d0"}
		return
	}
	var keys []string
	for p := d; p != nil; p = p.Parent {
		keys = append([]string{scanner.DirRow(p).Key}, keys...)
	}
	u.crumbs = keys
}

// ---------------------------------------------------------------- panel

func (u *UI) sidePanel(gtx layout.Context, r image.Rectangle) {
	d := func(v float32) int { return u.dp(gtx, v) }
	fill(gtx, r, u.pal.Surface)
	// tabs
	tabsH := d(40)
	labels := []string{u.t("tabMap"), u.t("tabTypes"), u.t("tabTop"), u.t("tabAge")}
	x := r.Min.X + d(8)
	for i, l := range labels {
		w := u.textWidth(gtx, l, 13, font.SemiBold) + d(24)
		tr := image.Rect(x, r.Min.Y+d(4), x+w, r.Min.Y+tabsH)
		if u.tabBtns[i].Clicked(gtx) && u.tab != i {
			u.tab = i
			u.requestSide()
		}
		i := i
		within(gtx, tr, func(gtx layout.Context) {
			u.tabBtns[i].Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				sz := gtx.Constraints.Min
				col, wt := u.pal.Muted, font.Normal
				if u.tab == i {
					col, wt = u.pal.Ink, font.SemiBold
					fill(gtx, image.Rect(0, sz.Y-d(2), sz.X, sz.Y), u.pal.Accent)
				} else if u.tabBtns[i].Hovered() {
					col = u.pal.Ink
				}
				u.text(gtx, image.Rectangle{Max: sz}, l, 13, col, wt, text.Middle)
				pointer.CursorPointer.Add(gtx.Ops)
				return layout.Dimensions{Size: sz}
			})
		})
		x += w + d(2)
	}
	fill(gtx, image.Rect(r.Min.X, r.Min.Y+tabsH, r.Max.X, r.Min.Y+tabsH+1), u.pal.Line)

	// breadcrumbs
	cy := r.Min.Y + tabsH + 1
	crH := d(32)
	u.layoutCrumbs(gtx, image.Rect(r.Min.X+d(10), cy, r.Max.X-d(10), cy+crH))
	body := image.Rect(r.Min.X, cy+crH, r.Max.X, r.Max.Y)
	switch u.tab {
	case 0:
		u.mapPanel(gtx, body.Inset(0), body)
	case 1:
		u.typesPanel(gtx, body)
	case 2:
		u.topPanel(gtx, body)
	case 3:
		u.agesPanel(gtx, body)
	}
}

func (u *UI) layoutCrumbs(gtx layout.Context, r image.Rectangle) {
	d := func(v float32) int { return u.dp(gtx, v) }
	if len(u.crumbBtns) < len(u.crumbs) {
		u.crumbBtns = make([]widget.Clickable, len(u.crumbs)+4)
	}
	x := r.Min.X
	for i, k := range u.crumbs {
		name := ""
		if i == 0 {
			name = u.tree.RootPath
		} else if dd, _, ok := u.tree.Resolve(k); ok {
			name = dd.Name
		}
		last := i == len(u.crumbs)-1
		wt := font.Normal
		col := u.pal.Muted
		if last {
			wt, col = font.SemiBold, u.pal.Ink
		}
		w := u.textWidth(gtx, name, 12, wt) + d(10)
		if w > d(220) {
			w = d(220)
		}
		if x+w > r.Max.X {
			break
		}
		if i > 0 {
			u.text(gtx, image.Rect(x, r.Min.Y, x+d(12), r.Max.Y), "›", 12, u.pal.Faint, font.Normal, text.Middle)
			x += d(12)
		}
		br := image.Rect(x, r.Min.Y+d(5), x+w, r.Max.Y-d(5))
		k := k
		if u.crumbBtns[i].Clicked(gtx) {
			u.reveal(k, true)
		}
		within(gtx, br, func(gtx layout.Context) {
			u.crumbBtns[i].Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				sz := gtx.Constraints.Min
				if u.crumbBtns[i].Hovered() {
					fillRR(gtx, image.Rectangle{Max: sz}, d(4), u.pal.Hover)
				}
				u.text(gtx, image.Rect(d(5), 0, sz.X-d(5), sz.Y), name, 12, col, wt, text.Start)
				pointer.CursorPointer.Add(gtx.Ops)
				return layout.Dimensions{Size: sz}
			})
		})
		x += w
	}
}

// ---------------------------------------------------------------- treemap panel

func (u *UI) mapPanel(gtx layout.Context, _ image.Rectangle, r image.Rectangle) {
	d := func(v float32) int { return u.dp(gtx, v) }
	legendH := d(46)
	if u.settings.MapColor == "depth" {
		legendH = d(8)
	}
	mr := image.Rect(r.Min.X+d(10), r.Min.Y, r.Max.X-d(10), r.Max.Y-legendH)
	within(gtx, mr, func(gtx layout.Context) { u.mapView.Layout(gtx, u, mr.Min) })
	if u.settings.MapColor != "depth" {
		x, y := mr.Min.X, mr.Max.Y+d(8)
		lh := d(18)
		items := categories
		for _, c := range items {
			lbl := u.t("cat_" + c)
			w := u.textWidth(gtx, lbl, 12, font.Normal) + d(18) + d(14)
			if x+w > mr.Max.X {
				x = mr.Min.X
				y += lh
			}
			fillRR(gtx, image.Rect(x, y+d(4), x+d(10), y+d(14)), d(2), catColors[c])
			u.text(gtx, image.Rect(x+d(16), y, x+w, y+lh), lbl, 12, u.pal.Muted, font.Normal, text.Start)
			x += w
		}
	}
}

// ---------------------------------------------------------------- types panel

func (u *UI) typesPanel(gtx layout.Context, r image.Rectangle) {
	d := func(v float32) int { return u.dp(gtx, v) }
	u.side.mu.Lock()
	exts := u.side.exts
	u.side.mu.Unlock()
	pad := d(14)
	if len(exts) == 0 {
		u.text(gtx, image.Rect(r.Min.X+pad, r.Min.Y+d(10), r.Max.X, r.Min.Y+d(40)), u.t("emptyFolder"), 13, u.pal.Muted, font.Normal, text.Start)
		return
	}
	var total int64
	cats := map[string]int64{}
	for _, x := range exts {
		total += x.Size
		c := "other"
		if x.Ext != "*" {
			c = catOf(x.Ext)
		}
		cats[c] += x.Size
	}
	if total <= 0 {
		total = 1
	}
	order := []string{}
	for _, c := range categories {
		if cats[c] > 0 {
			order = append(order, c)
		}
	}
	sort.SliceStable(order, func(i, j int) bool { return cats[order[i]] > cats[order[j]] })

	// strata bar
	y := r.Min.Y + d(4)
	bar := image.Rect(r.Min.X+pad, y, r.Max.X-pad, y+d(26))
	within(gtx, bar, func(gtx layout.Context) {
		fillRR(gtx, image.Rectangle{Max: bar.Size()}, d(5), u.pal.BarTrack)
		x := 0
		for i, c := range order {
			w := int(float64(bar.Dx()) * float64(cats[c]) / float64(total))
			if i == len(order)-1 {
				w = bar.Dx() - x
			}
			fill(gtx, image.Rect(x, 0, x+w, bar.Dy()), catColors[c])
			x += w
		}
	})
	y = bar.Max.Y + d(8)
	x := bar.Min.X
	lh := d(20)
	for _, c := range order {
		lbl := u.t("cat_" + c)
		p := u.pct(float64(cats[c]) * 100 / float64(total))
		w := u.textWidth(gtx, lbl, 12, font.Normal) + u.textWidth(gtx, p, 12, font.SemiBold) + d(36)
		if x+w > bar.Max.X {
			x = bar.Min.X
			y += lh
		}
		fillRR(gtx, image.Rect(x, y+d(5), x+d(10), y+d(15)), d(2), catColors[c])
		lw := u.text(gtx, image.Rect(x+d(16), y, x+w, y+lh), lbl, 12, u.pal.Muted, font.Normal, text.Start)
		u.text(gtx, image.Rect(x+d(20)+lw, y, x+w, y+lh), p, 12, u.pal.Ink, font.SemiBold, text.Start)
		x += w
	}
	y += lh + d(10)

	// table
	head := image.Rect(r.Min.X+pad, y, r.Max.X-pad, y+d(28))
	// type, category, count, size, share; the type column gets at least 110dp
	cw := []int{0, d(140), d(78), d(86), d(124)}
	w := head.Dx() - d(10)
	for _, i := range []int{1, 4} {
		if fixed := cw[1] + cw[2] + cw[3] + cw[4]; w-fixed < d(110) {
			cw[i] -= min(cw[i]-d(70), d(110)-(w-fixed))
		}
	}
	fixed := cw[1] + cw[2] + cw[3] + cw[4]
	xs := []int{0, w - fixed, w - fixed + cw[1], w - fixed + cw[1] + cw[2], w - fixed + cw[1] + cw[2] + cw[3], w}
	hl := []string{u.t("colType"), "", u.t("colCount"), u.t("colSize"), u.t("colPct")}
	for i, l := range hl {
		al := text.End
		if i < 2 {
			al = text.Start
		}
		u.text(gtx, image.Rect(head.Min.X+xs[i]+d(6), head.Min.Y, head.Min.X+xs[i+1]-d(6), head.Max.Y), l, 12, u.pal.Muted, font.Normal, al)
	}
	fill(gtx, image.Rect(head.Min.X, head.Max.Y-1, head.Max.X, head.Max.Y), u.pal.Line)
	lr := image.Rect(head.Min.X, head.Max.Y, head.Max.X, r.Max.Y)
	rowH := d(28)
	maxSize := exts[0].Size
	if maxSize <= 0 {
		maxSize = 1
	}
	within(gtx, lr, func(gtx layout.Context) {
		size := lr.Size()
		for _, e := range u.typesList.Update(gtx, len(exts), rowH, size) {
			if e.Kind == pointer.Press && e.Buttons == pointer.ButtonPrimary && e.Row >= 0 && exts[e.Row].Ext != "*" {
				f := exts[e.Row].Ext
				if f == "" {
					f = "-"
				}
				u.topFilter = f
				u.tab = 2
				u.requestSide()
			}
		}
		u.typesList.Layout(gtx, u, size, len(exts), rowH, func(gtx layout.Context, i int, rr image.Rectangle) {
			x := exts[i]
			if i == u.typesList.Hover && x.Ext != "*" {
				fill(gtx, rr, u.pal.Hover)
			}
			fill(gtx, image.Rect(rr.Min.X, rr.Max.Y-1, rr.Max.X, rr.Max.Y), u.pal.LineSoft)
			c := "other"
			if x.Ext != "*" {
				c = catOf(x.Ext)
			}
			lbl := "." + x.Ext
			if x.Ext == "*" {
				lbl = u.t("restExt")
			} else if x.Ext == "" {
				lbl = u.t("noExt")
			}
			cell := func(i int) image.Rectangle {
				return image.Rect(xs[i]+d(6), rr.Min.Y, xs[i+1]-d(6), rr.Max.Y)
			}
			cy := rr.Min.Y + rowH/2
			fillRR(gtx, image.Rect(d(6), cy-d(5), d(16), cy+d(5)), d(2), catColors[c])
			c0 := cell(0)
			c0.Min.X += d(18)
			u.text(gtx, c0, lbl, 13, u.pal.Ink, font.Normal, text.Start)
			u.text(gtx, cell(1), u.t("cat_"+c), 13, u.pal.Muted, font.Normal, text.Start)
			u.text(gtx, cell(2), u.num(x.Count), 13, u.pal.Ink, font.Normal, text.End)
			u.text(gtx, cell(3), u.size(x.Size), 13, u.pal.Ink, font.Normal, text.End)
			c4 := cell(4)
			numW := d(52)
			u.text(gtx, image.Rect(c4.Max.X-numW, c4.Min.Y, c4.Max.X, c4.Max.Y), u.pct(float64(x.Size)*100/float64(total)), 13, u.pal.Ink, font.Normal, text.End)
			mb := image.Rect(c4.Min.X, cy-d(3), c4.Max.X-numW-d(8), cy+d(3))
			if mb.Dx() > 8 {
				fillRR(gtx, mb, d(2), u.pal.BarTrack)
				fw := int(float64(mb.Dx()) * float64(x.Size) / float64(maxSize))
				fill(gtx, image.Rect(mb.Min.X, mb.Min.Y, mb.Min.X+fw, mb.Max.Y), catColors[c])
			}
			if x.Ext != "*" && i == u.typesList.Hover {
				pointer.CursorPointer.Add(gtx.Ops)
			}
		})
	})
}

// ---------------------------------------------------------------- largest files

func (u *UI) topPanel(gtx layout.Context, r image.Rectangle) {
	d := func(v float32) int { return u.dp(gtx, v) }
	u.side.mu.Lock()
	files := u.side.top
	u.side.mu.Unlock()
	pad := d(14)
	y := r.Min.Y
	if u.topFilter != "" {
		lbl := u.t("filterExt", "ext", u.topFilter)
		if u.topFilter == "-" {
			lbl = u.t("filterNoExt")
		}
		w := u.textWidth(gtx, lbl, 12, font.Normal) + d(40)
		cr := image.Rect(r.Min.X+pad, y+d(4), r.Min.X+pad+w, y+d(28))
		if u.chipBtn.Clicked(gtx) {
			u.topFilter = ""
			u.requestSide()
		}
		within(gtx, cr, func(gtx layout.Context) {
			u.chipBtn.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				sz := gtx.Constraints.Min
				fillRR(gtx, image.Rectangle{Max: sz}, sz.Y/2, u.pal.AccentSoft)
				u.text(gtx, image.Rect(d(12), 0, sz.X-d(26), sz.Y), lbl, 12, u.pal.Ink, font.Normal, text.Start)
				is := d(14)
				u.icon(gtx, ic.close, image.Rect(sz.X-d(22), (sz.Y-is)/2, sz.X-d(22)+is, (sz.Y+is)/2), u.pal.Muted)
				pointer.CursorPointer.Add(gtx.Ops)
				return layout.Dimensions{Size: sz}
			})
		})
		y = cr.Max.Y + d(6)
	}
	if len(files) == 0 {
		u.text(gtx, image.Rect(r.Min.X+pad, y+d(10), r.Max.X, y+d(40)), u.t("emptyFolder"), 13, u.pal.Muted, font.Normal, text.Start)
		return
	}
	head := image.Rect(r.Min.X+pad, y, r.Max.X-pad, y+d(28))
	sizeW, dateW := d(96), d(130)
	nameR := func(rr image.Rectangle) image.Rectangle {
		return image.Rect(d(6), rr.Min.Y, rr.Max.X-sizeW-dateW, rr.Max.Y)
	}
	u.text(gtx, image.Rect(head.Min.X+d(6), head.Min.Y, head.Max.X-sizeW-dateW, head.Max.Y), u.t("colName"), 12, u.pal.Muted, font.Normal, text.Start)
	u.text(gtx, image.Rect(head.Max.X-sizeW-dateW, head.Min.Y, head.Max.X-dateW-d(6), head.Max.Y), u.t("colSize"), 12, u.pal.Muted, font.Normal, text.End)
	u.text(gtx, image.Rect(head.Max.X-dateW, head.Min.Y, head.Max.X-d(6), head.Max.Y), u.t("colMtime"), 12, u.pal.Muted, font.Normal, text.End)
	fill(gtx, image.Rect(head.Min.X, head.Max.Y-1, head.Max.X, head.Max.Y), u.pal.Line)
	lr := image.Rect(head.Min.X, head.Max.Y, head.Max.X, r.Max.Y)
	rowH := d(42)
	root := u.tree.RootPath
	within(gtx, lr, func(gtx layout.Context) {
		size := lr.Size()
		for _, e := range u.topList.Update(gtx, len(files), rowH, size) {
			if e.Kind != pointer.Press || e.Row < 0 {
				continue
			}
			f := files[e.Row]
			if e.Buttons == pointer.ButtonSecondary {
				u.selected = f.Key
				u.itemMenu(lr.Min.Add(e.Pos.Round()), f.Key, "top")
				continue
			}
			dbl := u.lastPress.row == -100-e.Row && e.Time-u.lastPress.at < 450*time.Millisecond
			u.lastPress.row, u.lastPress.at = -100-e.Row, e.Time
			if dbl {
				u.openItem(f.Key)
			} else {
				u.reveal(f.Key, false)
			}
		}
		u.topList.Layout(gtx, u, size, len(files), rowH, func(gtx layout.Context, i int, rr image.Rectangle) {
			f := files[i]
			if f.Key == u.selected {
				fill(gtx, rr, u.pal.Sel)
			} else if i == u.topList.Hover {
				fill(gtx, rr, u.pal.Hover)
			}
			fill(gtx, image.Rect(rr.Min.X, rr.Max.Y-1, rr.Max.X, rr.Max.Y), u.pal.LineSoft)
			nr := nameR(rr)
			top := image.Rect(nr.Min.X+d(18), rr.Min.Y+d(4), nr.Max.X, rr.Min.Y+d(23))
			fillRR(gtx, image.Rect(nr.Min.X, top.Min.Y+d(5), nr.Min.X+d(10), top.Min.Y+d(15)), d(2), catColors[catOf(f.Ext)])
			u.text(gtx, top, f.Name, 13, u.pal.Ink, font.Normal, text.Start)
			rel := f.Folder
			if len(rel) >= len(root) && rel[:len(root)] == root {
				rel = "…" + rel[len(root):]
				if rel == "…" {
					rel = root
				}
			}
			u.text(gtx, image.Rect(top.Min.X, top.Max.Y, nr.Max.X, rr.Max.Y-d(4)), rel, 11, u.pal.Muted, font.Normal, text.Start)
			u.text(gtx, image.Rect(rr.Max.X-sizeW-dateW, rr.Min.Y, rr.Max.X-dateW-d(6), rr.Max.Y), u.size(f.Size), 13, u.pal.Ink, font.Normal, text.End)
			u.text(gtx, image.Rect(rr.Max.X-dateW, rr.Min.Y, rr.Max.X-d(6), rr.Max.Y), u.date(f.MTime), 12, u.pal.Muted, font.Normal, text.End)
		})
	})
}

// ---------------------------------------------------------------- ages

func (u *UI) agesPanel(gtx layout.Context, r image.Rectangle) {
	d := func(v float32) int { return u.dp(gtx, v) }
	u.side.mu.Lock()
	ages := u.side.ages
	u.side.mu.Unlock()
	pad := d(14)
	var total, max int64
	for _, a := range ages {
		total += a.Size
		if a.Size > max {
			max = a.Size
		}
	}
	if total == 0 {
		u.text(gtx, image.Rect(r.Min.X+pad, r.Min.Y+d(10), r.Max.X, r.Min.Y+d(40)), u.t("emptyFolder"), 13, u.pal.Muted, font.Normal, text.Start)
		return
	}
	area := image.Rect(r.Min.X+pad, r.Min.Y+d(24), r.Max.X-pad, r.Min.Y+d(244))
	n := len(ages)
	gap := d(10)
	w := (area.Dx() - gap*(n-1)) / n
	for i, a := range ages {
		x := area.Min.X + i*(w+gap)
		h := int(float64(area.Dy()-d(24)) * float64(a.Size) / float64(max))
		if h < d(2) {
			h = d(2)
		}
		br := image.Rect(x, area.Max.Y-h, x+w, area.Max.Y)
		fillRR(gtx, br, d(4), u.pal.Accent)
		fill(gtx, image.Rect(br.Min.X, br.Max.Y-d(4), br.Max.X, br.Max.Y), u.pal.Accent)
		u.text(gtx, image.Rect(x-d(10), br.Min.Y-d(22), x+w+d(10), br.Min.Y-d(4)), u.size(a.Size), 12, u.pal.Ink, font.SemiBold, text.Middle)
		u.text(gtx, image.Rect(x-d(6), area.Max.Y+d(6), x+w+d(6), area.Max.Y+d(24)), u.t("age_"+a.ID), 12, u.pal.Muted, font.Normal, text.Middle)
		fl := u.t("filesN", "n", u.num(a.Count))
		if a.Count == 1 {
			fl = u.t("files1")
		}
		u.text(gtx, image.Rect(x-d(6), area.Max.Y+d(22), x+w+d(6), area.Max.Y+d(38)), fl, 11, u.pal.Faint, font.Normal, text.Middle)
	}
	fill(gtx, image.Rect(area.Min.X, area.Max.Y, area.Max.X, area.Max.Y+1), u.pal.Line)
	u.wrapText(gtx, image.Pt(area.Min.X, area.Max.Y+d(52)), min(area.Dx(), d(560)), u.t("ageNote"), 12, u.pal.Muted)
}

var _ color.NRGBA
