package app

import (
	"math"

	"github.com/hajimehoshi/ebiten/v2/vector"
)

func cos(v float64) float64 { return math.Cos(v) }
func sin(v float64) float64 { return math.Sin(v) }

func (c *canvas) line(x0, y0, x1, y1, w float64, col rgb, a float32) {
	vector.StrokeLine(c.dst, c.px(x0), c.px(y0), c.px(x1), c.px(y1), c.px(w), col.rgba(a), true)
}

func (in *input) anyUp() bool {
	for i := range in.ptrs {
		if in.ptrs[i].up && !in.ptrs[i].consumed {
			return true
		}
	}
	return false
}

// consumeAll makes current pointers inert (after a screen change, so the
// finger that pressed a button does not also act on the next screen).
func (in *input) consumeAll() {
	for i := range in.ptrs {
		in.ptrs[i].consumed = true
	}
}

// ── widgets (immediate mode: the same call lays out, hit-tests and draws) ──

type btnStyle int

const (
	btnPrimary btnStyle = iota
	btnSecondary
	btnGhost
)

// button draws a rounded button and reports a tap. Pass c == nil to only
// hit-test (Update) and in == nil to only draw.
func (a *App) button(c *canvas, label string, x, y, w, h float64, st btnStyle) bool {
	if c == nil {
		if a.in.tapped(x, y, w, h) {
			a.sfx(sfxTap, 0.6)
			a.haptic(HapticLight)
			return true
		}
		return false
	}
	press := a.in.pressing(x, y, w, h) != nil
	k := 1.0
	if press {
		k = 0.96
	}
	cx, cy := x+w/2, y+h/2
	w2, h2 := w*k, h*k
	x2, y2 := cx-w2/2, cy-h2/2
	r := h2 / 2
	switch st {
	case btnPrimary:
		c.round(x2, y2+4, w2, h2, r, accent2.scale(0.55), 1)
		c.round(x2, y2, w2, h2, r, accent2, 1)
		c.round(x2+2.5, y2+2.5, w2-5, h2-5, r-2.5, accent, 1)
		c.round(x2+8, y2+4, w2-16, h2*0.42, (h2*0.42)/2, rgb{1, 1, 1}, 0.22)
		c.text(label, cx, cy+1, h*0.36, true, hex(0x3a2a00), 1, 1)
	case btnSecondary:
		c.round(x2, y2+3, w2, h2, r, rgb{}, 0.3)
		c.round(x2, y2, w2, h2, r, cardBg.lighten(0.06), 1)
		c.roundStroke(x2+0.5, y2+0.5, w2-1, h2-1, r, 1.4, cardEdge.lighten(0.15), 1)
		c.text(label, cx, cy+1, h*0.34, true, textCol, 1, 1)
	default:
		c.text(label, cx, cy, h*0.36, true, dimText, 1, 1)
	}
	return false
}

// iconButton is a round button with a glyph drawn by fn.
func (a *App) iconButton(c *canvas, x, y, d float64, fn func(c *canvas, cx, cy, r float64)) bool {
	if c == nil {
		if a.in.tapped(x-6, y-6, d+12, d+12) {
			a.sfx(sfxTap, 0.6)
			a.haptic(HapticLight)
			return true
		}
		return false
	}
	press := a.in.pressing(x-6, y-6, d+12, d+12) != nil
	r := d / 2
	if press {
		r *= 0.92
	}
	c.circle(x+d/2, y+d/2+2, r, rgb{}.rgba(0.3))
	c.circle(x+d/2, y+d/2, r, cardBg.lighten(0.08).rgba(1))
	fn(c, x+d/2, y+d/2, r)
	return false
}

func pauseGlyph(c *canvas, cx, cy, r float64) {
	c.round(cx-r*0.32, cy-r*0.38, r*0.22, r*0.76, r*0.08, textCol, 1)
	c.round(cx+r*0.10, cy-r*0.38, r*0.22, r*0.76, r*0.08, textCol, 1)
}

func backGlyph(c *canvas, cx, cy, r float64) {
	c.line(cx+r*0.15, cy-r*0.35, cx-r*0.2, cy, r*0.16, textCol, 1)
	c.line(cx-r*0.2, cy, cx+r*0.15, cy+r*0.35, r*0.16, textCol, 1)
}

// toggle is a switch row; it returns the new value.
func (a *App) toggle(c *canvas, label string, x, y, w float64, on bool) bool {
	const h = 46.0
	if c == nil {
		if a.in.tapped(x, y, w, h) {
			a.sfx(sfxTap, 0.6)
			a.haptic(HapticLight)
			return !on
		}
		return on
	}
	c.text(label, x+16, y+h/2, 17, false, textCol, 1, 0)
	tw, th := 52.0, 30.0
	tx, ty := x+w-tw-14, y+(h-th)/2
	bg := cardEdge
	if on {
		bg = accent2
	}
	c.round(tx, ty, tw, th, th/2, bg, 1)
	kx := tx + 3
	if on {
		kx = tx + tw - th + 3
	}
	c.circle(kx+(th-6)/2, ty+th/2, (th-6)/2, textCol.rgba(1))
	return on
}

// slider is a 0…1 value row.
func (a *App) slider(c *canvas, label string, x, y, w, v float64) float64 {
	const h = 60.0
	sx, sw := x+16, w-32
	sy := y + 40
	if c == nil {
		if p := a.in.pressing(x, y+18, w, h-12); p != nil && !p.consumed {
			v = clamp01((p.x - sx) / sw)
		}
		return v
	}
	c.text(label, x+16, y+18, 17, false, textCol, 1, 0)
	c.text(pct(v), x+w-16, y+18, 15, false, dimText, 1, 2)
	c.round(sx, sy-3, sw, 6, 3, cardEdge, 1)
	c.round(sx, sy-3, sw*v, 6, 3, accent2, 1)
	c.circle(sx+sw*v, sy, 11, textCol.rgba(1))
	return v
}

func pct(v float64) string {
	n := int(v*100 + 0.5)
	return itoa(n) + "%"
}

// segmented picks one of opts; returns the index.
func (a *App) segmented(c *canvas, label string, x, y, w float64, opts []string, sel int) int {
	const h = 84.0
	bw := (w - 32) / float64(len(opts))
	by := y + 36
	if c == nil {
		for i := range opts {
			if a.in.tapped(x+16+float64(i)*bw, by, bw, 38) {
				a.sfx(sfxTap, 0.6)
				a.haptic(HapticLight)
				return i
			}
		}
		return sel
	}
	c.text(label, x+16, y+18, 17, false, textCol, 1, 0)
	c.round(x+16, by, w-32, 38, 19, cardEdge.scale(0.8), 1)
	c.round(x+16+float64(sel)*bw+2, by+2, bw-4, 34, 17, accent2, 1)
	for i, o := range opts {
		col := textCol
		if i == sel {
			col = hex(0x3a2a00)
		}
		c.text(o, x+16+float64(i)*bw+bw/2, by+19, 14.5, true, col, 1, 1)
	}
	_ = h
	return sel
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}

// grouped formats 12345 as "12,345".
func grouped(n int) string {
	s := itoa(n)
	if len(s) <= 3 || n < 0 {
		return s
	}
	out := make([]byte, 0, len(s)+len(s)/3)
	for i, ch := range []byte(s) {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, ch)
	}
	return string(out)
}

// card is a rounded panel.
func (c *canvas) card(x, y, w, h float64) {
	c.round(x, y+4, w, h, 22, rgb{}, 0.35)
	c.round(x, y, w, h, 22, cardBg, 0.96)
	c.roundStroke(x+0.5, y+0.5, w-1, h-1, 22, 1.2, cardEdge, 1)
}
