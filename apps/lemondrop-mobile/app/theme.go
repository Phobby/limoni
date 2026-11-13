package app

import (
	"bytes"
	"image"
	"image/color"
	"math"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/gomedium"
)

// Palette: Lemon Drop's own colours (apps/lemondrop/render.go), brightened
// for a lit GPU board. Index 0 is unused; 1–4 lemon, lime, grapefruit,
// blueberry.
var (
	fruit = [5]rgb{{}, hex(0xf6d32a), hex(0x7ccb48), hex(0xf0625f), hex(0x5f86ea)}

	bgTop     = hex(0x1b1530)
	bgBottom  = hex(0x0b0d16)
	boardBg   = hex(0x141720)
	boardEdge = hex(0xf2c200)
	textCol   = hex(0xf4f1e6)
	dimText   = hex(0x9a98a6)
	accent    = hex(0xf5d000)
	accent2   = hex(0xffb020)
	danger    = hex(0xff5a6a)
	cardBg    = hex(0x1e2130)
	cardEdge  = hex(0x343a52)
	rindCol   = hex(0xf2c200)
	pithCol   = hex(0xfff3c4)
	fleshCol  = hex(0xffd84a)
)

type rgb struct{ r, g, b float32 }

func hex(v uint32) rgb {
	return rgb{float32(v>>16&0xff) / 255, float32(v>>8&0xff) / 255, float32(v&0xff) / 255}
}

func (c rgb) mix(o rgb, t float32) rgb {
	return rgb{c.r + (o.r-c.r)*t, c.g + (o.g-c.g)*t, c.b + (o.b-c.b)*t}
}
func (c rgb) scale(k float32) rgb   { return rgb{c.r * k, c.g * k, c.b * k} }
func (c rgb) lighten(t float32) rgb { return c.mix(rgb{1, 1, 1}, t) }

func (c rgb) rgba(a float32) color.RGBA {
	cl := func(v float32) uint8 { return uint8(max(0, min(1, v))*a*255 + 0.5) }
	return color.RGBA{cl(c.r), cl(c.g), cl(c.b), uint8(max(0, min(1, a))*255 + 0.5)}
}

func (c rgb) cs(a float32) ebiten.ColorScale {
	var s ebiten.ColorScale
	s.Scale(c.r*a, c.g*a, c.b*a, a)
	return s
}

// ── fonts ────────────────────────────────────────────────────────────────

var (
	fontBold, fontMed *text.GoTextFaceSource
	faces             = map[int]*text.GoTextFace{}
)

func init() {
	var err error
	if fontBold, err = text.NewGoTextFaceSource(bytes.NewReader(gobold.TTF)); err != nil {
		panic(err)
	}
	if fontMed, err = text.NewGoTextFaceSource(bytes.NewReader(gomedium.TTF)); err != nil {
		panic(err)
	}
}

// face is a cached face at a pixel size (bold when bold).
func face(px float64, bold bool) *text.GoTextFace {
	k := int(px*4) << 1
	if bold {
		k |= 1
	}
	if f, ok := faces[k]; ok {
		return f
	}
	src := fontMed
	if bold {
		src = fontBold
	}
	f := &text.GoTextFace{Source: src, Size: px}
	faces[k] = f
	return f
}

// ── canvas: drawing in dp ────────────────────────────────────────────────

// canvas draws in density-independent points; s converts to pixels.
type canvas struct {
	dst   *ebiten.Image
	saved *ebiten.Image
	s     float32
	path  vector.Path
}

func (c *canvas) px(v float64) float32 { return float32(v) * c.s }

func (c *canvas) rect(x, y, w, h float64, col color.RGBA) {
	vector.FillRect(c.dst, c.px(x), c.px(y), c.px(w), c.px(h), col, false)
}

func (c *canvas) circle(x, y, r float64, col color.RGBA) {
	vector.FillCircle(c.dst, c.px(x), c.px(y), c.px(r), col, true)
}

func (c *canvas) roundPath(x, y, w, h, r float64) *vector.Path {
	p := &c.path
	p.Reset()
	X, Y, W, H, R := c.px(x), c.px(y), c.px(w), c.px(h), c.px(min(r, w/2, h/2))
	p.MoveTo(X+R, Y)
	p.LineTo(X+W-R, Y)
	p.Arc(X+W-R, Y+R, R, -math.Pi/2, 0, vector.Clockwise)
	p.LineTo(X+W, Y+H-R)
	p.Arc(X+W-R, Y+H-R, R, 0, math.Pi/2, vector.Clockwise)
	p.LineTo(X+R, Y+H)
	p.Arc(X+R, Y+H-R, R, math.Pi/2, math.Pi, vector.Clockwise)
	p.LineTo(X, Y+R)
	p.Arc(X+R, Y+R, R, math.Pi, 3*math.Pi/2, vector.Clockwise)
	p.Close()
	return p
}

func (c *canvas) round(x, y, w, h, r float64, col rgb, a float32) {
	op := &vector.DrawPathOptions{AntiAlias: true, ColorScale: col.cs(a)}
	vector.FillPath(c.dst, c.roundPath(x, y, w, h, r), &vector.FillOptions{}, op)
}

func (c *canvas) roundStroke(x, y, w, h, r, width float64, col rgb, a float32) {
	op := &vector.DrawPathOptions{AntiAlias: true, ColorScale: col.cs(a)}
	vector.StrokePath(c.dst, c.roundPath(x, y, w, h, r), &vector.StrokeOptions{Width: c.px(width), LineJoin: vector.LineJoinRound}, op)
}

// text draws s with its anchor at (x, y): align 0 left, 1 centre, 2 right;
// y is the vertical centre of the line.
func (c *canvas) text(s string, x, y, size float64, bold bool, col rgb, a float32, align int) {
	f := face(size*float64(c.s), bold)
	op := &text.DrawOptions{}
	op.GeoM.Translate(float64(c.px(x)), float64(c.px(y)))
	op.ColorScale = col.cs(a)
	op.PrimaryAlign = text.Align(align)
	op.SecondaryAlign = text.AlignCenter
	text.Draw(c.dst, s, f, op)
}

// textScaled draws s centred at (x, y) scaled by k around its centre (for pops).
func (c *canvas) textScaled(s string, x, y, size, k float64, col rgb, a float32) {
	f := face(size*float64(c.s), true)
	op := &text.DrawOptions{}
	op.GeoM.Scale(k, k)
	op.GeoM.Translate(float64(c.px(x)), float64(c.px(y)))
	op.ColorScale = col.cs(a)
	op.PrimaryAlign = text.AlignCenter
	op.SecondaryAlign = text.AlignCenter
	text.Draw(c.dst, s, f, op)
}

func textWidth(s string, size float64, bold bool, scale float32) float64 {
	w, _ := text.Measure(s, face(size*float64(scale), bold), 0)
	return w / float64(scale)
}

// ── easing ───────────────────────────────────────────────────────────────

func clamp01(t float64) float64 { return max(0, min(1, t)) }
func easeOutCubic(t float64) float64 {
	t = clamp01(t)
	return 1 - (1-t)*(1-t)*(1-t)
}
func easeOutBack(t float64) float64 {
	t = clamp01(t)
	const c1, c3 = 1.70158, 2.70158
	return 1 + c3*math.Pow(t-1, 3) + c1*math.Pow(t-1, 2)
}
func easeInOut(t float64) float64 {
	t = clamp01(t)
	return t * t * (3 - 2*t)
}
func lerp(a, b, t float64) float64 { return a + (b-a)*t }

// clip limits drawing to a rectangle (dp) until unclip.
func (c *canvas) clip(x, y, w, h float64) {
	c.saved = c.dst
	r := image.Rect(int(c.px(x)), int(c.px(y)), int(c.px(x+w)+1), int(c.px(y+h)+1))
	c.dst = c.dst.SubImage(r).(*ebiten.Image)
}

func (c *canvas) unclip() {
	if c.saved != nil {
		c.dst, c.saved = c.saved, nil
	}
}

func nowUnix() int64 { return time.Now().Unix() }
