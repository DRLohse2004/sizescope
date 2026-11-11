package main

import (
	"image"
	"image/color"
	"math"
	"time"

	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"golang.org/x/exp/shiny/materialdesign/icons"
)

func mustIcon(data []byte) *widget.Icon {
	ic, err := widget.NewIcon(data)
	if err != nil {
		panic(err)
	}
	return ic
}

var ic = struct {
	folder, file, chevron, chevronDown, drive, net, usb, pick, export, shield, gear, play, stop,
	open, copy, refresh, target, trash, mapIc, cloud, close, check, arrowDown, arrowUp *widget.Icon
}{
	folder: mustIcon(icons.FileFolder), file: mustIcon(icons.EditorInsertDriveFile),
	chevron: mustIcon(icons.NavigationChevronRight), chevronDown: mustIcon(icons.NavigationExpandMore),
	drive: mustIcon(icons.DeviceStorage), net: mustIcon(icons.FileCloudQueue), usb: mustIcon(icons.DeviceUSB),
	pick: mustIcon(icons.FileFolderOpen), export: mustIcon(icons.FileFileDownload), shield: mustIcon(icons.HardwareSecurity),
	gear: mustIcon(icons.ActionSettings), play: mustIcon(icons.AVPlayArrow), stop: mustIcon(icons.AVStop),
	open: mustIcon(icons.ActionOpenInNew), copy: mustIcon(icons.ContentContentCopy), refresh: mustIcon(icons.NavigationRefresh),
	target: mustIcon(icons.MapsMyLocation), trash: mustIcon(icons.ActionDelete), mapIc: mustIcon(icons.ActionDashboard),
	cloud: mustIcon(icons.FileCloud), close: mustIcon(icons.NavigationClose), check: mustIcon(icons.NavigationCheck),
	arrowDown: mustIcon(icons.NavigationArrowDropDown), arrowUp: mustIcon(icons.NavigationArrowDropUp),
}

// ---------------------------------------------------------------- pointer helpers

func pointerEvents(gtx layout.Context, tag event.Tag, kinds pointer.Kind) []pointer.Event {
	var out []pointer.Event
	f := pointer.Filter{Target: tag, Kinds: kinds}
	if kinds&pointer.Scroll != 0 {
		f.ScrollY = pointer.ScrollRange{Min: math.MinInt32, Max: math.MaxInt32}
		f.ScrollX = pointer.ScrollRange{Min: math.MinInt32, Max: math.MaxInt32}
	}
	for {
		ev, ok := gtx.Event(f)
		if !ok {
			break
		}
		if pe, ok := ev.(pointer.Event); ok {
			out = append(out, pe)
		}
	}
	return out
}

// hitArea registers tag for pointer input over r (in current coordinates).
func hitArea(gtx layout.Context, r image.Rectangle, tag event.Tag, cursor pointer.Cursor) {
	cl := clip.Rect(r).Push(gtx.Ops)
	event.Op(gtx.Ops, tag)
	if cursor != pointer.CursorDefault {
		cursor.Add(gtx.Ops)
	}
	cl.Pop()
}

// ---------------------------------------------------------------- virtual list

// VList is a vertically scrolling list of equally tall rows with a scrollbar.
type VList struct {
	off      int
	tag      int8
	barTag   int8
	dragging bool
	dragY    float32
	dragOff  int
	Hover    int // hovered row, -1 if none
	barHover bool
}

// RowEvent is a pointer event mapped to a row.
type RowEvent struct {
	Kind    pointer.Kind
	Buttons pointer.Buttons
	Row     int
	Pos     f32.Point // relative to the list viewport
	Mods    key.Modifiers
	Time    time.Duration
}

// Update processes input and returns the row events for the caller.
func (v *VList) Update(gtx layout.Context, n, rowH int, size image.Point) []RowEvent {
	var out []RowEvent
	maxOff := n*rowH - size.Y
	if maxOff < 0 {
		maxOff = 0
	}
	for _, e := range pointerEvents(gtx, &v.tag, pointer.Press|pointer.Release|pointer.Move|pointer.Leave|pointer.Scroll|pointer.Drag) {
		switch e.Kind {
		case pointer.Scroll:
			v.off += int(e.Scroll.Y)
		case pointer.Leave:
			v.Hover = -1
		default:
			row := (int(e.Position.Y) + v.off) / rowH
			if row >= n || e.Position.Y < 0 {
				row = -1
			}
			if e.Kind == pointer.Move {
				v.Hover = row
			}
			out = append(out, RowEvent{e.Kind, e.Buttons, row, e.Position, e.Modifiers, e.Time})
		}
	}
	trackH := float32(size.Y)
	thumbH := float32(size.Y)
	if n*rowH > 0 {
		thumbH = trackH * float32(size.Y) / float32(n*rowH)
	}
	if thumbH < 24 {
		thumbH = 24
	}
	for _, e := range pointerEvents(gtx, &v.barTag, pointer.Press|pointer.Drag|pointer.Release|pointer.Enter|pointer.Leave) {
		switch e.Kind {
		case pointer.Enter:
			v.barHover = true
		case pointer.Leave:
			v.barHover = false
		case pointer.Press:
			v.dragging, v.dragY, v.dragOff = true, e.Position.Y, v.off
			// click on the track jumps there
			thumbY := float32(0)
			if maxOff > 0 {
				thumbY = (trackH - thumbH) * float32(v.off) / float32(maxOff)
			}
			if e.Position.Y < thumbY || e.Position.Y > thumbY+thumbH {
				if trackH > thumbH {
					v.off = int((e.Position.Y - thumbH/2) / (trackH - thumbH) * float32(maxOff))
				}
				v.dragOff = v.off
			}
		case pointer.Drag:
			if v.dragging && trackH > thumbH {
				v.off = v.dragOff + int((e.Position.Y-v.dragY)/(trackH-thumbH)*float32(maxOff))
			}
		case pointer.Release:
			v.dragging = false
		}
	}
	v.clamp(maxOff)
	return out
}

func (v *VList) clamp(maxOff int) {
	if v.off > maxOff {
		v.off = maxOff
	}
	if v.off < 0 {
		v.off = 0
	}
}

// EnsureVisible scrolls so that row i is visible.
func (v *VList) EnsureVisible(i, rowH, height int) {
	top := i * rowH
	if top < v.off {
		v.off = top
	} else if top+rowH > v.off+height {
		v.off = top + rowH - height + rowH
	}
}

// Layout draws the visible rows and the scrollbar.
func (v *VList) Layout(gtx layout.Context, u *UI, size image.Point, n, rowH int, row func(gtx layout.Context, i int, r image.Rectangle)) {
	first := v.off / rowH
	last := (v.off + size.Y) / rowH
	if last >= n {
		last = n - 1
	}
	cl := clip.Rect(image.Rectangle{Max: size}).Push(gtx.Ops)
	rowW := size.X
	if n*rowH > size.Y {
		rowW -= u.dp(gtx, 10) // keep content clear of the scrollbar
	}
	for i := first; i <= last && i >= 0; i++ {
		y := i*rowH - v.off
		row(gtx, i, image.Rect(0, y, rowW, y+rowH))
	}
	hitArea(gtx, image.Rectangle{Max: size}, &v.tag, pointer.CursorDefault)
	if n*rowH > size.Y {
		bw := u.dp(gtx, 10)
		track := image.Rect(size.X-bw, 0, size.X, size.Y)
		thumbH := size.Y * size.Y / (n * rowH)
		if min := u.dp(gtx, 24); thumbH < min {
			thumbH = min
		}
		maxOff := n*rowH - size.Y
		y := 0
		if maxOff > 0 {
			y = (size.Y - thumbH) * v.off / maxOff
		}
		col := u.pal.Line
		if v.barHover || v.dragging {
			col = u.pal.Faint
		}
		m := u.dp(gtx, 2)
		fillRR(gtx, image.Rect(track.Min.X+m, y+m, track.Max.X-m, y+thumbH-m), u.dp(gtx, 3), col)
		hitArea(gtx, track, &v.barTag, pointer.CursorDefault)
	}
	cl.Pop()
}

// ---------------------------------------------------------------- buttons

type btnKind int

const (
	btnPlain btnKind = iota
	btnPrimary
	btnDanger
	btnGhost
)

// button draws a text button (optionally with icon) filling r.
func (u *UI) button(gtx layout.Context, c *widget.Clickable, r image.Rectangle, label string, icon *widget.Icon, kind btnKind) bool {
	clicked := c.Clicked(gtx)
	within(gtx, r, func(gtx layout.Context) {
		c.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			sz := gtx.Constraints.Min
			rr := image.Rectangle{Max: sz}
			rad := u.dp(gtx, 6)
			bg, fg := u.pal.Raised, u.pal.Ink
			switch kind {
			case btnPrimary:
				bg, fg = u.pal.Accent, u.pal.AccentInk
			case btnDanger:
				bg, fg = u.pal.Danger, rgb(0xffffff)
			case btnGhost:
				bg = color.NRGBA{}
			}
			if c.Hovered() {
				if kind == btnPlain || kind == btnGhost {
					bg = u.pal.Hover
				} else {
					bg = lighten(bg, 18)
				}
			}
			fillRR(gtx, rr, rad, bg)
			if kind == btnPlain {
				strokeRR(gtx, rr, rad, 1, u.pal.Line)
			}
			w := font.Normal
			if kind == btnPrimary || kind == btnDanger {
				w = font.SemiBold
			}
			tw := u.textWidth(gtx, label, 13, w)
			isz := 0
			if icon != nil {
				isz = u.dp(gtx, 16)
			}
			gap := 0
			if icon != nil && label != "" {
				gap = u.dp(gtx, 6)
			}
			x := (sz.X - tw - isz - gap) / 2
			if icon != nil {
				u.icon(gtx, icon, image.Rect(x, (sz.Y-isz)/2, x+isz, (sz.Y+isz)/2), fg)
			}
			u.text(gtx, image.Rect(x+isz+gap, 0, x+isz+gap+tw+2, sz.Y), label, 13, fg, w, text.Start)
			pointer.CursorPointer.Add(gtx.Ops)
			return layout.Dimensions{Size: sz}
		})
	})
	return clicked
}

// iconButton draws a square icon-only button.
func (u *UI) iconButton(gtx layout.Context, c *widget.Clickable, r image.Rectangle, icon *widget.Icon, col color.NRGBA, enabled bool) bool {
	clicked := c.Clicked(gtx) && enabled
	within(gtx, r, func(gtx layout.Context) {
		c.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			sz := gtx.Constraints.Min
			if c.Hovered() && enabled {
				fillRR(gtx, image.Rectangle{Max: sz}, u.dp(gtx, 6), u.pal.Hover)
				col = u.pal.Ink
			}
			if !enabled {
				col.A = 90
			}
			isz := u.dp(gtx, 18)
			u.icon(gtx, icon, image.Rect((sz.X-isz)/2, (sz.Y-isz)/2, (sz.X+isz)/2, (sz.Y+isz)/2), col)
			if enabled {
				pointer.CursorPointer.Add(gtx.Ops)
			}
			return layout.Dimensions{Size: sz}
		})
	})
	return clicked
}

func lighten(c color.NRGBA, d int) color.NRGBA {
	f := func(v uint8) uint8 {
		n := int(v) + d
		if n > 255 {
			n = 255
		}
		if n < 0 {
			n = 0
		}
		return uint8(n)
	}
	return color.NRGBA{R: f(c.R), G: f(c.G), B: f(c.B), A: c.A}
}

// checkbox draws a toggle box at r.Min; returns true when toggled.
func (u *UI) checkbox(gtx layout.Context, c *widget.Clickable, r image.Rectangle, on *bool) {
	if c.Clicked(gtx) {
		*on = !*on
	}
	within(gtx, r, func(gtx layout.Context) {
		c.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			s := u.dp(gtx, 18)
			y := (gtx.Constraints.Min.Y - s) / 2
			box := image.Rect(0, y, s, y+s)
			if *on {
				fillRR(gtx, box, u.dp(gtx, 4), u.pal.Accent)
				m := u.dp(gtx, 2)
				u.icon(gtx, ic.check, box.Inset(m), u.pal.AccentInk)
			} else {
				fillRR(gtx, box, u.dp(gtx, 4), u.pal.Surface)
				strokeRR(gtx, box, u.dp(gtx, 4), float32(u.dp(gtx, 1.5)), u.pal.Faint)
			}
			pointer.CursorPointer.Add(gtx.Ops)
			return layout.Dimensions{Size: gtx.Constraints.Min}
		})
	})
}

// segmented draws a group of options; sel is the selected value.
func (u *UI) segmented(gtx layout.Context, btns []widget.Clickable, r image.Rectangle, values, labels []string, sel *string) {
	for i := range values {
		if btns[i].Clicked(gtx) {
			*sel = values[i]
		}
	}
	within(gtx, r, func(gtx layout.Context) {
		sz := gtx.Constraints.Min
		rad := u.dp(gtx, 6)
		fillRR(gtx, image.Rectangle{Max: sz}, rad, u.pal.Surface)
		strokeRR(gtx, image.Rectangle{Max: sz}, rad, 1, u.pal.Line)
		w := sz.X / len(values)
		for i := range values {
			cell := image.Rect(i*w, 0, (i+1)*w, sz.Y)
			if i == len(values)-1 {
				cell.Max.X = sz.X
			}
			within(gtx, cell, func(gtx layout.Context) {
				btns[i].Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					c := gtx.Constraints.Min
					m := u.dp(gtx, 3)
					col := u.pal.Muted
					if values[i] == *sel {
						fillRR(gtx, image.Rect(m, m, c.X-m, c.Y-m), u.dp(gtx, 4), u.pal.Accent)
						col = u.pal.AccentInk
					} else if btns[i].Hovered() {
						fillRR(gtx, image.Rect(m, m, c.X-m, c.Y-m), u.dp(gtx, 4), u.pal.Hover)
						col = u.pal.Ink
					}
					u.text(gtx, image.Rect(m, 0, c.X-m, c.Y), labels[i], 13, col, font.Normal, text.Middle)
					pointer.CursorPointer.Add(gtx.Ops)
					return layout.Dimensions{Size: c}
				})
			})
		}
	})
}

// shadowBox draws a raised panel with border and soft shadow.
func (u *UI) shadowBox(gtx layout.Context, r image.Rectangle, rad int) {
	for i := 3; i >= 1; i-- {
		s := u.dp(gtx, float32(i)*2)
		c := u.pal.Shadow
		c.A = uint8(int(c.A) / (i + 1))
		fillRR(gtx, image.Rect(r.Min.X-s/2, r.Min.Y, r.Max.X+s/2, r.Max.Y+s), rad+s/2, c)
	}
	fillRR(gtx, r, rad, u.pal.Raised)
	strokeRR(gtx, r, rad, 1, u.pal.Line)
}

// ---------------------------------------------------------------- menu

type menuItem struct {
	icon   *widget.Icon
	label  string
	hint   string
	danger bool
	sep    bool
	action func()
}

type Menu struct {
	at    image.Point
	items []menuItem
	btns  []widget.Clickable
	tag   int8
}

func (u *UI) openMenu(at image.Point, items []menuItem) {
	u.menu = &Menu{at: at, items: items, btns: make([]widget.Clickable, len(items))}
}

func (u *UI) layoutMenu(gtx layout.Context) {
	m := u.menu
	full := image.Rectangle{Max: gtx.Constraints.Max}
	// click outside closes
	for _, e := range pointerEvents(gtx, &m.tag, pointer.Press) {
		_ = e
		u.menu = nil
		return
	}
	hitArea(gtx, full, &m.tag, pointer.CursorDefault)

	itemH, sepH, pad := u.dp(gtx, 32), u.dp(gtx, 9), u.dp(gtx, 4)
	w := u.dp(gtx, 250)
	h := pad * 2
	for _, it := range m.items {
		if it.sep {
			h += sepH
		} else {
			h += itemH
		}
	}
	x, y := m.at.X, m.at.Y
	if x+w > full.Max.X-8 {
		x = full.Max.X - 8 - w
	}
	if y+h > full.Max.Y-8 {
		y = full.Max.Y - 8 - h
	}
	box := image.Rect(x, y, x+w, y+h)
	u.shadowBox(gtx, box, u.dp(gtx, 8))
	cy := y + pad
	for i := range m.items {
		it := m.items[i]
		if it.sep {
			fill(gtx, image.Rect(x+pad, cy+sepH/2, x+w-pad, cy+sepH/2+1), u.pal.LineSoft)
			cy += sepH
			continue
		}
		r := image.Rect(x+pad, cy, x+w-pad, cy+itemH)
		if m.btns[i].Clicked(gtx) {
			u.menu = nil
			if it.action != nil {
				it.action()
			}
			return
		}
		within(gtx, r, func(gtx layout.Context) {
			m.btns[i].Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				sz := gtx.Constraints.Min
				if m.btns[i].Hovered() {
					fillRR(gtx, image.Rectangle{Max: sz}, u.dp(gtx, 5), u.pal.Hover)
				}
				col := u.pal.Ink
				if it.danger {
					col = u.pal.Danger
				}
				is := u.dp(gtx, 16)
				px := u.dp(gtx, 10)
				if it.icon != nil {
					icol := u.pal.Muted
					if it.danger {
						icol = u.pal.Danger
					}
					u.icon(gtx, it.icon, image.Rect(px, (sz.Y-is)/2, px+is, (sz.Y+is)/2), icol)
				}
				tx := px + is + u.dp(gtx, 10)
				u.text(gtx, image.Rect(tx, 0, sz.X-px, sz.Y), it.label, 13, col, font.Normal, text.Start)
				if it.hint != "" {
					u.text(gtx, image.Rect(tx, 0, sz.X-px, sz.Y), it.hint, 11, u.pal.Faint, font.Normal, text.End)
				}
				return layout.Dimensions{Size: sz}
			})
		})
		cy += itemH
	}
}

// ---------------------------------------------------------------- toasts & tooltip

type toast struct {
	msg   string
	err   bool
	until time.Time
}

func (u *UI) toast(msg string, isErr bool) {
	d := 3200 * time.Millisecond
	if isErr {
		d = 6 * time.Second
	}
	u.toasts = append(u.toasts, toast{msg, isErr, time.Now().Add(d)})
	if u.win != nil {
		time.AfterFunc(d+50*time.Millisecond, u.win.Invalidate)
		u.win.Invalidate()
	}
}

func (u *UI) layoutToasts(gtx layout.Context) {
	now := time.Now()
	keep := u.toasts[:0]
	for _, t := range u.toasts {
		if now.Before(t.until) {
			keep = append(keep, t)
		}
	}
	u.toasts = keep
	full := gtx.Constraints.Max
	w := u.dp(gtx, 420)
	y := full.Y - u.dp(gtx, 40)
	for i := len(u.toasts) - 1; i >= 0; i-- {
		t := u.toasts[i]
		inner := w - u.dp(gtx, 30)
		g := gtx
		m := op.Record(gtx.Ops)
		th := u.wrapText(g, image.Point{}, inner, t.msg, 13, u.pal.Ink)
		m.Stop()
		h := th + u.dp(gtx, 20)
		r := image.Rect(full.X-u.dp(gtx, 16)-w, y-h, full.X-u.dp(gtx, 16), y)
		u.shadowBox(gtx, r, u.dp(gtx, 8))
		bar := u.pal.Accent
		if t.err {
			bar = u.pal.Danger
		}
		fill(gtx, image.Rect(r.Min.X, r.Min.Y+u.dp(gtx, 6), r.Min.X+u.dp(gtx, 3), r.Max.Y-u.dp(gtx, 6)), bar)
		u.wrapText(gtx, image.Pt(r.Min.X+u.dp(gtx, 16), r.Min.Y+u.dp(gtx, 10)), inner, t.msg, 13, u.pal.Ink)
		y = r.Min.Y - u.dp(gtx, 8)
	}
}

type tipInfo struct {
	at    image.Point
	title string
	lines []string
}

func (u *UI) layoutTip(gtx layout.Context) {
	t := u.tip
	if t == nil {
		return
	}
	full := gtx.Constraints.Max
	pad := u.dp(gtx, 10)
	w := u.textWidth(gtx, t.title, 13, font.SemiBold)
	for _, l := range t.lines {
		if lw := u.textWidth(gtx, l, 12, font.Normal); lw > w {
			w = lw
		}
	}
	if max := u.dp(gtx, 420); w > max {
		w = max
	}
	lh := u.dp(gtx, 18)
	h := pad*2 + lh*(1+len(t.lines))
	x, y := t.at.X+u.dp(gtx, 14), t.at.Y+u.dp(gtx, 16)
	if x+w+2*pad > full.X-8 {
		x = t.at.X - w - 2*pad - u.dp(gtx, 8)
	}
	if y+h > full.Y-8 {
		y = t.at.Y - h - u.dp(gtx, 8)
	}
	r := image.Rect(x, y, x+w+2*pad, y+h)
	u.shadowBox(gtx, r, u.dp(gtx, 6))
	u.text(gtx, image.Rect(r.Min.X+pad, r.Min.Y+pad, r.Max.X-pad, r.Min.Y+pad+lh), t.title, 13, u.pal.Ink, font.SemiBold, text.Start)
	for i, l := range t.lines {
		yy := r.Min.Y + pad + lh*(i+1)
		u.text(gtx, image.Rect(r.Min.X+pad, yy, r.Max.X-pad, yy+lh), l, 12, u.pal.Muted, font.Normal, text.Start)
	}
}

var _ = unit.Dp(0)
