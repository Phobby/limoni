package app

import (
	"math"

	"github.com/hajimehoshi/ebiten/v2"
)

// Particles live in a fixed pool and are drawn in one DrawTriangles call per
// blend mode, from one soft-disc texture. Nothing here allocates per frame.

const maxParticles = 3000

type particle struct {
	x, y, vx, vy  float32 // dp, dp/s
	life, maxLife float32
	size, grow    float32
	col           rgb
	alpha         float32
	gravity, drag float32
	additive      bool
}

type fxSystem struct {
	p      [maxParticles]particle
	n      int
	disc   *ebiten.Image
	verts  []ebiten.Vertex
	idx    []uint16
	reduce bool
	seed   uint32

	texts [12]floatText
	shake float64
}

type floatText struct {
	s      string
	x, y   float64
	size   float64
	col    rgb
	t, dur float64
	rise   float64
	pop    bool
}

func newFX() *fxSystem {
	f := &fxSystem{seed: 0x9e3779b9}
	const n = 32
	img := ebiten.NewImage(n, n)
	px := make([]byte, n*n*4)
	for y := 0; y < n; y++ {
		for x := 0; x < n; x++ {
			d := math.Hypot(float64(x)+0.5-n/2, float64(y)+0.5-n/2) / (n / 2)
			a := math.Max(0, 1-d)
			a = a * a * (3 - 2*a)
			v := uint8(a * 255)
			i := (y*n + x) * 4
			px[i], px[i+1], px[i+2], px[i+3] = v, v, v, v
		}
	}
	img.WritePixels(px)
	f.disc = img
	f.verts = make([]ebiten.Vertex, 0, maxParticles*4)
	f.idx = make([]uint16, 0, maxParticles*6)
	return f
}

func (f *fxSystem) rnd() float32 {
	f.seed ^= f.seed << 13
	f.seed ^= f.seed >> 17
	f.seed ^= f.seed << 5
	return float32(f.seed) / float32(math.MaxUint32)
}

func (f *fxSystem) add(p particle) {
	if f.reduce && f.n > maxParticles/4 {
		return
	}
	if f.n == maxParticles {
		return
	}
	f.p[f.n] = p
	f.n++
}

// burst throws n particles from (x, y) in dp.
func (f *fxSystem) burst(x, y float64, n int, col rgb, speed, size float32, additive bool) {
	if f.reduce {
		n = (n + 3) / 4
	}
	for i := 0; i < n; i++ {
		a := f.rnd() * 2 * math.Pi
		v := speed * (0.3 + 0.7*f.rnd())
		f.add(particle{
			x: float32(x), y: float32(y),
			vx: v * float32(math.Cos(float64(a))), vy: v*float32(math.Sin(float64(a))) - speed*0.3,
			life: 0, maxLife: 0.45 + 0.5*f.rnd(),
			size: size * (0.6 + 0.8*f.rnd()), grow: -0.6,
			col: col.lighten(0.15 * f.rnd()), alpha: 1, gravity: 520, drag: 1.6, additive: additive,
		})
	}
}

// dust is a sand grain thrown loose: falls, fades.
func (f *fxSystem) dust(x, y float64, col rgb, size float32) {
	f.add(particle{
		x: float32(x), y: float32(y),
		vx: (f.rnd() - 0.5) * 140, vy: -60 - 160*f.rnd(),
		maxLife: 0.6 + 0.6*f.rnd(), size: size * (0.8 + 0.5*f.rnd()), grow: -0.3,
		col: col, alpha: 1, gravity: 700, drag: 0.8,
	})
}

// sparkle is a glint that rises.
func (f *fxSystem) sparkle(x, y float64, col rgb, size float32) {
	f.add(particle{
		x: float32(x), y: float32(y),
		vx: (f.rnd() - 0.5) * 60, vy: -40 - 90*f.rnd(),
		maxLife: 0.5 + 0.5*f.rnd(), size: size * (0.7 + f.rnd()), grow: 0.4,
		col: col.lighten(0.5), alpha: 0.9, gravity: -40, drag: 1.2, additive: true,
	})
}

func (f *fxSystem) say(s string, x, y, size float64, col rgb, dur float64, pop bool) {
	slot := 0
	for i := range f.texts {
		if f.texts[i].s == "" || f.texts[i].t >= f.texts[i].dur {
			slot = i
			break
		}
		if f.texts[i].t > f.texts[slot].t {
			slot = i
		}
	}
	f.texts[slot] = floatText{s: s, x: x, y: y, size: size, col: col, dur: dur, rise: 40, pop: pop}
}

func (f *fxSystem) kick(amount float64) {
	if !f.reduce {
		f.shake = max(f.shake, amount)
	}
}

func (f *fxSystem) update(dt float64) {
	d := float32(dt)
	for i := 0; i < f.n; {
		p := &f.p[i]
		p.life += d
		if p.life >= p.maxLife {
			f.n--
			f.p[i] = f.p[f.n]
			continue
		}
		p.vy += p.gravity * d
		k := 1 - p.drag*d
		p.vx *= k
		p.vy *= k
		p.x += p.vx * d
		p.y += p.vy * d
		i++
	}
	for i := range f.texts {
		if f.texts[i].s != "" {
			f.texts[i].t += dt
		}
	}
	f.shake = max(0, f.shake-dt*30)
}

func (f *fxSystem) shakeOffset(t float64) (float64, float64) {
	if f.shake <= 0 {
		return 0, 0
	}
	return math.Sin(t*71) * f.shake, math.Cos(t*53) * f.shake * 0.7
}

// draw renders the particles: normal ones, then additive glints.
func (f *fxSystem) draw(c *canvas, ox, oy float64) {
	for pass := 0; pass < 2; pass++ {
		f.verts, f.idx = f.verts[:0], f.idx[:0]
		for i := 0; i < f.n; i++ {
			p := &f.p[i]
			if p.additive != (pass == 1) {
				continue
			}
			t := p.life / p.maxLife
			a := p.alpha * (1 - t*t)
			sz := p.size * max(0.05, 1+p.grow*t) * c.s
			x, y := (p.x+float32(ox))*c.s, (p.y+float32(oy))*c.s
			r, g, b := p.col.r*a, p.col.g*a, p.col.b*a
			base := uint16(len(f.verts))
			const T = 32
			f.verts = append(f.verts,
				ebiten.Vertex{DstX: x - sz, DstY: y - sz, SrcX: 0, SrcY: 0, ColorR: r, ColorG: g, ColorB: b, ColorA: a},
				ebiten.Vertex{DstX: x + sz, DstY: y - sz, SrcX: T, SrcY: 0, ColorR: r, ColorG: g, ColorB: b, ColorA: a},
				ebiten.Vertex{DstX: x - sz, DstY: y + sz, SrcX: 0, SrcY: T, ColorR: r, ColorG: g, ColorB: b, ColorA: a},
				ebiten.Vertex{DstX: x + sz, DstY: y + sz, SrcX: T, SrcY: T, ColorR: r, ColorG: g, ColorB: b, ColorA: a},
			)
			f.idx = append(f.idx, base, base+1, base+2, base+1, base+3, base+2)
		}
		if len(f.idx) == 0 {
			continue
		}
		op := &ebiten.DrawTrianglesOptions{}
		if pass == 1 {
			op.Blend = ebiten.BlendLighter
		}
		c.dst.DrawTriangles(f.verts, f.idx, f.disc, op)
	}
}

func (f *fxSystem) drawTexts(c *canvas) {
	for i := range f.texts {
		t := &f.texts[i]
		if t.s == "" || t.t >= t.dur {
			continue
		}
		u := t.t / t.dur
		a := float32(1 - math.Pow(u, 3))
		k := 1.0
		if t.pop && !f.reduce {
			k = easeOutBack(t.t / 0.25)
		}
		y := t.y - t.rise*easeOutCubic(u)
		c.textScaled(t.s, t.x+1.5, y+2, t.size, k, rgb{}, a*0.5)
		c.textScaled(t.s, t.x, y, t.size, k, t.col, a)
	}
}

// counter animates a displayed number towards its real value.
type counter struct{ shown, target float64 }

func (c *counter) set(v int)  { c.target = float64(v) }
func (c *counter) snap(v int) { c.shown, c.target = float64(v), float64(v) }
func (c *counter) value() int { return int(c.shown + 0.5) }
func (c *counter) update(dt float64) {
	d := c.target - c.shown
	if math.Abs(d) < 0.5 {
		c.shown = c.target
		return
	}
	c.shown += d * min(1, dt*9)
}
