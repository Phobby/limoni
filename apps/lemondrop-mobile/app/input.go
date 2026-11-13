package app

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

// pointer is a finger or the mouse, in dp. Touch and mouse feed the same
// gesture code, so the desktop build exercises the phone controls.
type pointer struct {
	id       int // touch id, or -1 for the mouse
	x, y     float64
	sx, sy   float64 // where it went down
	px, py   float64 // last frame
	vx, vy   float64 // dp/s, smoothed
	t0, t    float64 // down time, now
	down, up bool    // went down / came up this frame
	active   bool
	consumed bool // a gesture has taken it (e.g. a hard drop fired)
	dragging bool
}

func (p *pointer) dx() float64   { return p.x - p.sx }
func (p *pointer) dy() float64   { return p.y - p.sy }
func (p *pointer) held() float64 { return p.t - p.t0 }

type input struct {
	ptrs    [6]pointer
	touches []ebiten.TouchID
	now     float64
}

// update reads touches and the mouse; scale converts pixels to dp.
func (in *input) update(now, scale, dt float64) {
	in.now = now
	for i := range in.ptrs {
		p := &in.ptrs[i]
		p.down = false
		if p.up {
			p.active, p.up = false, false
		}
	}
	in.touches = ebiten.AppendTouchIDs(in.touches[:0])
	seen := 0
	for _, id := range in.touches {
		x, y := ebiten.TouchPosition(id)
		in.track(int(id), float64(x)/scale, float64(y)/scale, dt)
		seen++
	}
	for i := range in.ptrs {
		p := &in.ptrs[i]
		if p.active && p.id >= 0 && !p.up && !in.hasTouch(p.id) {
			p.up = true
		}
	}
	if seen == 0 {
		mx, my := ebiten.CursorPosition()
		if ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
			in.track(-1, float64(mx)/scale, float64(my)/scale, dt)
		} else if inpututil.IsMouseButtonJustReleased(ebiten.MouseButtonLeft) {
			for i := range in.ptrs {
				if in.ptrs[i].active && in.ptrs[i].id == -1 {
					in.ptrs[i].up = true
				}
			}
		}
	}
}

func (in *input) hasTouch(id int) bool {
	for _, t := range in.touches {
		if int(t) == id {
			return true
		}
	}
	return false
}

func (in *input) track(id int, x, y, dt float64) {
	for i := range in.ptrs {
		p := &in.ptrs[i]
		if p.active && p.id == id && !p.up {
			p.px, p.py = p.x, p.y
			p.x, p.y, p.t = x, y, in.now
			if dt > 0 {
				k := min(1, dt*18)
				p.vx += ((x-p.px)/dt - p.vx) * k
				p.vy += ((y-p.py)/dt - p.vy) * k
			}
			return
		}
	}
	for i := range in.ptrs {
		p := &in.ptrs[i]
		if !p.active {
			*p = pointer{id: id, x: x, y: y, sx: x, sy: y, px: x, py: y, t0: in.now, t: in.now, down: true, active: true}
			return
		}
	}
}

// tapped reports a pointer that came up this frame inside the rect, having
// gone down inside it and not travelled far: a button press.
func (in *input) tapped(x, y, w, h float64) bool {
	for i := range in.ptrs {
		p := &in.ptrs[i]
		if p.up && !p.consumed && inside(p.sx, p.sy, x, y, w, h) && inside(p.x, p.y, x-12, y-12, w+24, h+24) &&
			abs(p.dx())+abs(p.dy()) < 30 {
			p.consumed = true
			return true
		}
	}
	return false
}

// pressing reports a pointer held down inside the rect (for hold buttons).
func (in *input) pressing(x, y, w, h float64) *pointer {
	for i := range in.ptrs {
		p := &in.ptrs[i]
		if p.active && !p.up && inside(p.x, p.y, x, y, w, h) {
			return p
		}
	}
	return nil
}

func inside(px, py, x, y, w, h float64) bool { return px >= x && py >= y && px < x+w && py < y+h }

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

// keys is the desktop keyboard, for development and for desktops.
type keys struct{}

func (keys) just(k ebiten.Key) bool { return inpututil.IsKeyJustPressed(k) }
func (keys) down(k ebiten.Key) bool { return ebiten.IsKeyPressed(k) }
func (keys) repeat(k ebiten.Key) bool {
	d := inpututil.KeyPressDuration(k)
	return d == 1 || d > 10 && (d-10)%3 == 0
}
