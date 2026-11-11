package main

import (
	"image"
	"image/color"
	"math"
	"time"

	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"

	"sizescope/scanner"
)

// MapView draws a squarified, nested treemap.
type MapView struct {
	data      *scanner.MapNode
	dirty     bool
	size      image.Point
	rects     []mapRect
	hover     int
	selKey    string
	tag       int8
	lastClick struct {
		idx int
		at  time.Duration
	}
	branches map[string]int
	pos      image.Point
}

type mapRect struct {
	n      *scanner.MapNode
	r      image.Rectangle
	depth  int
	branch int
	folder bool // drawn as frame with children
	header bool
}

var depthHues = []float64{205, 28, 150, 265, 45, 340, 180, 95, 0, 230}

func (m *MapView) Layout(gtx layout.Context, u *UI, origin image.Point) {
	size := gtx.Constraints.Max
	if m.dirty || size != m.size {
		m.size = size
		m.dirty = false
		m.relayout(u, gtx)
	}

	// input
	for _, e := range pointerEvents(gtx, &m.tag, pointer.Move|pointer.Leave|pointer.Press) {
		switch e.Kind {
		case pointer.Leave:
			m.hover = -1
		case pointer.Move:
			m.hover = m.hit(e.Position)
			m.pos = e.Position.Round()
		case pointer.Press:
			i := m.hit(e.Position)
			if i < 0 {
				continue
			}
			n := m.rects[i].n
			if e.Buttons == pointer.ButtonSecondary {
				if n.Key != "" {
					u.itemMenu(origin.Add(e.Position.Round()), n.Key, "map")
				}
				continue
			}
			dbl := m.lastClick.idx == i && e.Time-m.lastClick.at < 450*time.Millisecond
			m.lastClick.idx, m.lastClick.at = i, e.Time
			if n.Key == "" {
				continue
			}
			if dbl && n.Dir {
				u.setFocus(n.Key)
			} else {
				u.reveal(n.Key, false)
			}
		}
	}

	fill(gtx, image.Rectangle{Max: size}, mapFolderColor(&u.pal, 0))
	for i := range m.rects {
		m.paint(gtx, u, &m.rects[i])
	}
	// selection and hover outlines
	for i := range m.rects {
		if m.selKey != "" && m.rects[i].n.Key == m.selKey {
			strokeRect(gtx, m.rects[i].r, 3, rgb(0xffffff))
			strokeRect(gtx, m.rects[i].r, 1, rgba(0x000000, 170))
		}
	}
	if m.hover >= 0 && m.hover < len(m.rects) {
		hr := m.rects[m.hover]
		strokeRect(gtx, hr.r, 2, rgba(0xffffff, 230))
		n := hr.n
		title := n.Name
		if n.Rest > 0 {
			title = u.t("restItems", "n", u.num(int64(n.Rest)))
		}
		total := u.tree.Root.Size()
		share := 0.0
		if total > 0 {
			share = float64(n.Size) * 100 / float64(total)
		}
		lines := []string{u.size(n.Size) + "  (" + u.pct(share) + ")"}
		switch {
		case n.Dir:
			lines = append(lines, u.t("cat_folder"))
		case n.Rest == 0:
			l := u.t("cat_" + catOf(n.Ext))
			if n.On {
				l += ", " + u.t("onlineOnly")
			}
			lines = append(lines, l)
		}
		u.tip = &tipInfo{at: origin.Add(m.pos), title: title, lines: lines}
	}
	hitArea(gtx, image.Rectangle{Max: size}, &m.tag, pointer.CursorPointer)
}

func (m *MapView) hit(p f32.Point) int {
	pt := p.Round()
	for i := len(m.rects) - 1; i >= 0; i-- {
		if pt.In(m.rects[i].r) {
			return i
		}
	}
	return -1
}

func (m *MapView) relayout(u *UI, gtx layout.Context) {
	m.rects = m.rects[:0]
	m.hover = -1
	m.branches = map[string]int{}
	if m.data == nil || m.data.Size <= 0 {
		return
	}
	m.layoutNode(u, gtx, m.data, image.Rectangle{Max: m.size}, 0, -1, true)
}

func (m *MapView) layoutNode(u *UI, gtx layout.Context, n *scanner.MapNode, r image.Rectangle, depth, branch int, isRoot bool) {
	if r.Dx() < 1 || r.Dy() < 1 {
		return
	}
	kids := n.Kids
	inner := r
	if !isRoot {
		mr := mapRect{n: n, r: r, depth: depth, branch: branch, folder: len(kids) > 0}
		if len(kids) > 0 {
			head := 0
			if r.Dy() > u.dp(gtx, 44) && r.Dx() > u.dp(gtx, 70) {
				head = u.dp(gtx, 17)
				mr.header = true
			}
			pad := 0
			if r.Dx() > 12 && r.Dy() > 12 {
				pad = u.dp(gtx, 2)
			}
			inner = image.Rect(r.Min.X+pad, r.Min.Y+pad+head, r.Max.X-pad, r.Max.Y-pad)
		}
		m.rects = append(m.rects, mr)
		if len(kids) == 0 || inner.Dx() < 2 || inner.Dy() < 2 {
			return
		}
	}
	var total int64
	for _, k := range kids {
		if k.Size > 0 {
			total += k.Size
		}
	}
	if total <= 0 {
		return
	}
	scale := float64(inner.Dx()) * float64(inner.Dy()) / float64(total)
	items := make([]sqItem, 0, len(kids))
	for _, k := range kids {
		if k.Size > 0 {
			items = append(items, sqItem{k, float64(k.Size) * scale})
		}
	}
	squarify(items, float64(inner.Min.X), float64(inner.Min.Y), float64(inner.Dx()), float64(inner.Dy()), func(it sqItem, x, y, w, h float64) {
		rr := image.Rect(int(math.Round(x)), int(math.Round(y)), int(math.Round(x+w)), int(math.Round(y+h)))
		b := branch
		if isRoot {
			b = len(m.branches)
			m.branches[it.n.Key+it.n.Name] = b
		}
		m.layoutNode(u, gtx, it.n, rr, depth+1, b, false)
	})
}

type sqItem struct {
	n *scanner.MapNode
	a float64
}

func squarify(items []sqItem, x, y, w, h float64, place func(it sqItem, x, y, w, h float64)) {
	i := 0
	for i < len(items) {
		short := math.Min(w, h)
		if short <= 0 {
			return
		}
		sum := items[i].a
		worst := worstRatio(items[i:i+1], sum, short)
		j := i + 1
		for j < len(items) {
			ns := sum + items[j].a
			nw := worstRatio(items[i:j+1], ns, short)
			if nw > worst {
				break
			}
			sum, worst = ns, nw
			j++
		}
		thick := sum / short
		off := 0.0
		for _, it := range items[i:j] {
			l := it.a / thick
			if w >= h {
				place(it, x, y+off, thick, l)
			} else {
				place(it, x+off, y, l, thick)
			}
			off += l
		}
		if w >= h {
			x += thick
			w -= thick
		} else {
			y += thick
			h -= thick
		}
		i = j
	}
}

func worstRatio(row []sqItem, sum, short float64) float64 {
	mx, mn := 0.0, math.Inf(1)
	for _, r := range row {
		mx = math.Max(mx, r.a)
		mn = math.Min(mn, r.a)
	}
	s2, w2 := sum*sum, short*short
	return math.Max(w2*mx/s2, s2/(w2*mn))
}

func (m *MapView) tileColor(u *UI, mr *mapRect) color.NRGBA {
	n := mr.n
	if n.Rest > 0 {
		return catColors["other"]
	}
	if u.settings.MapColor == "depth" {
		hue := depthHues[(mr.branch%len(depthHues)+len(depthHues))%len(depthHues)]
		l := 0.52 - float64(mr.depth)*0.04
		if l < 0.3 {
			l = 0.3
		}
		return hsl(hue, 0.42, l)
	}
	if n.Dir {
		return catColors["other"]
	}
	return catColors[catOf(n.Ext)]
}

func (m *MapView) paint(gtx layout.Context, u *UI, mr *mapRect) {
	r := mr.r
	if mr.folder {
		fill(gtx, image.Rect(r.Min.X, r.Min.Y, r.Max.X, r.Max.Y), mapFolderColor(&u.pal, mr.depth))
		// thin separation from the neighbour
		strokeRect(gtx, r, 1, mapFolderColor(&u.pal, 0))
		if mr.header {
			lbl := mr.n.Name + "  " + u.size(mr.n.Size)
			u.text(gtx, image.Rect(r.Min.X+u.dp(gtx, 5), r.Min.Y+u.dp(gtx, 1), r.Max.X-u.dp(gtx, 5), r.Min.Y+u.dp(gtx, 17)), lbl, 11, u.pal.MapFolderInk, font.SemiBold, text.Start)
		}
		return
	}
	col := m.tileColor(u, mr)
	tile := r
	if r.Dx() > 3 {
		tile.Max.X--
	}
	if r.Dy() > 3 {
		tile.Max.Y--
	}
	if tile.Dx() <= 0 || tile.Dy() <= 0 {
		return
	}
	if tile.Dx() > 10 && tile.Dy() > 10 {
		// soft light from the top left gives tiles some depth
		st := clip.Rect(tile).Push(gtx.Ops)
		paint.LinearGradientOp{
			Stop1: f32.Pt(float32(tile.Min.X), float32(tile.Min.Y)), Color1: lighten(col, 22),
			Stop2: f32.Pt(float32(tile.Max.X), float32(tile.Max.Y)), Color2: lighten(col, -18),
		}.Add(gtx.Ops)
		paint.PaintOp{}.Add(gtx.Ops)
		st.Pop()
	} else {
		fill(gtx, tile, col)
	}
	if mr.n.Rest > 0 || mr.n.On {
		// hatch aggregated small items and online-only files
		st := clip.Rect(tile).Push(gtx.Ops)
		hc := rgba(0xffffff, 60)
		step := u.dp(gtx, 6)
		for k := -tile.Dy(); k < tile.Dx(); k += step {
			var p clip.Path
			p.Begin(gtx.Ops)
			p.MoveTo(f32.Pt(float32(tile.Min.X+k), float32(tile.Max.Y)))
			p.LineTo(f32.Pt(float32(tile.Min.X+k+tile.Dy()), float32(tile.Min.Y)))
			paint.FillShape(gtx.Ops, hc, clip.Stroke{Path: p.End(), Width: 1}.Op())
		}
		st.Pop()
	}
	if tile.Dx() > u.dp(gtx, 46) && tile.Dy() > u.dp(gtx, 18) {
		name := mr.n.Name
		if mr.n.Rest > 0 {
			name = u.t("restItems", "n", u.num(int64(mr.n.Rest)))
		}
		x0 := tile.Min.X + u.dp(gtx, 5)
		u.text(gtx, image.Rect(x0, tile.Min.Y+u.dp(gtx, 3), tile.Max.X-u.dp(gtx, 4), tile.Min.Y+u.dp(gtx, 18)), name, 11, rgba(0xffffff, 245), font.SemiBold, text.Start)
		if tile.Dy() > u.dp(gtx, 34) {
			u.text(gtx, image.Rect(x0, tile.Min.Y+u.dp(gtx, 17), tile.Max.X-u.dp(gtx, 4), tile.Min.Y+u.dp(gtx, 32)), u.size(mr.n.Size), 11, rgba(0xffffff, 195), font.Normal, text.Start)
		}
	}
}

func hsl(h, s, l float64) color.NRGBA {
	c := (1 - math.Abs(2*l-1)) * s
	hp := h / 60
	x := c * (1 - math.Abs(math.Mod(hp, 2)-1))
	var r, g, b float64
	switch {
	case hp < 1:
		r, g, b = c, x, 0
	case hp < 2:
		r, g, b = x, c, 0
	case hp < 3:
		r, g, b = 0, c, x
	case hp < 4:
		r, g, b = 0, x, c
	case hp < 5:
		r, g, b = x, 0, c
	default:
		r, g, b = c, 0, x
	}
	mm := l - c/2
	return color.NRGBA{R: uint8((r + mm) * 255), G: uint8((g + mm) * 255), B: uint8((b + mm) * 255), A: 255}
}
