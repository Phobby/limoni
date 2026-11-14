package app

import (
	"github.com/hajimehoshi/ebiten/v2"

	"github.com/thebanri/limoni/apps/lemondrop-mobile/core"
	"github.com/thebanri/limoni/apps/lemondrop-mobile/gen"
)

// dropPlay is a run of the game.
type dropPlay struct {
	a   *App
	sim *core.Sim
	tex *sandTex

	// layout, dp
	bx, by, bw, bh, g float64
	hudY, ctrlY       float64

	score counter
	start float64

	// gesture
	gp          *pointer
	anchorX     int
	anchorPiece int
	tapCand     bool

	// buttons / keys auto-repeat
	dasDir int
	dasT   float64

	// clear snapshot for vanish particles
	snap [1200]struct {
		x, y int32
		c    uint8
	}
	nsnap   int
	trickT  float64
	overT   float64
	banner  string
	bannerT float64
}

func (a *App) startDrop() {
	d := &dropPlay{a: a, tex: newSandTex()}
	d.sim = core.NewSim(a.nextSeed(), core.MaxB)
	d.sim.Gen = &gen.Director{}
	d.sim.Start()
	d.start = a.t
	a.drop = d
	a.paused = false
	a.go_(scrDrop)
	a.sfx(sfxStart, 0.8)
}

func (d *dropPlay) layout() {
	a := d.a
	sx, sy, sw, sh := a.safe()
	const hud = 92.0
	ctrl := 34.0
	if a.st.data.Settings.Buttons {
		ctrl = 104
	}
	availW, availH := sw-24, sh-hud-ctrl-16
	d.bw = min(availW, availH*float64(core.Cols)/float64(core.Rows))
	d.bh = d.bw * float64(core.Rows) / float64(core.Cols)
	d.g = d.bw / float64(d.sim.GW)
	d.bx = sx + (sw-d.bw)/2
	d.by = sy + hud + 8 + (availH-d.bh)/2
	d.hudY = sy
	d.ctrlY = sy + sh - ctrl
}

// ── update ───────────────────────────────────────────────────────────────

func (d *dropPlay) update(dt float64) {
	a := d.a
	d.layout()
	s := d.sim
	d.score.set(s.Score)
	d.score.update(dt)
	d.bannerT += dt
	if a.paused {
		d.updatePause()
		return
	}
	if a.iconButton(nil, d.pauseX(), d.hudY+14, 44, nil) || a.keys.just(ebiten.KeyP) || a.keys.just(ebiten.KeyEscape) {
		a.paused = true
		a.sfx(sfxPause, 0.7)
		return
	}
	if s.Phase == core.Playing {
		d.controls(dt)
	}
	s.Update(dt)
	d.events()
	if s.Phase == core.Over {
		d.overT += dt
		if d.overT > 1.4 {
			a.finishDrop(d)
		}
	}
}

func (d *dropPlay) pauseX() float64 {
	_, _, sw, _ := d.a.safe()
	sx, _, _, _ := d.a.safe()
	return sx + sw - 58
}

func (d *dropPlay) controls(dt float64) {
	a, s := d.a, d.sim
	k := a.keys
	soft := false
	// keyboard
	if k.repeat(ebiten.KeyArrowLeft) || k.repeat(ebiten.KeyA) {
		s.Shift(-s.Step())
	}
	if k.repeat(ebiten.KeyArrowRight) || k.repeat(ebiten.KeyD) {
		s.Shift(s.Step())
	}
	if k.just(ebiten.KeyArrowUp) || k.just(ebiten.KeyX) || k.just(ebiten.KeyW) {
		s.Turn(1)
	}
	if k.just(ebiten.KeyZ) {
		s.Turn(-1)
	}
	if k.just(ebiten.KeySpace) {
		s.HardDrop()
	}
	soft = k.down(ebiten.KeyArrowDown) || k.down(ebiten.KeyS)

	if a.st.data.Settings.Buttons {
		soft = d.buttonControls(dt) || soft
	} else {
		soft = d.gestures() || soft
	}
	s.SoftDrop = soft
}

// gestures: drag sideways to steer, tap to turn, flick down to drop, hold
// down to soft-drop. Returns whether soft drop is held.
func (d *dropPlay) gestures() bool {
	a, s := d.a, d.sim
	in := &a.in
	if d.gp != nil && (!d.gp.active || d.gp.up) {
		p := d.gp
		if d.tapCand && !p.consumed && p.held() < 0.3 && abs(p.dx())+abs(p.dy()) < 14 {
			dir := 1
			if p.x < a.W/2 {
				dir = -1
			}
			s.Turn(dir)
		}
		d.gp = nil
	}
	if d.gp == nil {
		for i := range in.ptrs {
			p := &in.ptrs[i]
			// a gesture starts below the HUD and above any button bar
			if p.down && !p.consumed && p.y > d.hudY+80 && p.y < d.ctrlY {
				d.gp, d.anchorX, d.anchorPiece, d.tapCand = p, s.Cur.X, s.Pieces, true
				break
			}
		}
	}
	p := d.gp
	if p == nil || p.consumed {
		return false
	}
	if s.Pieces != d.anchorPiece { // a new piece: keep steering it from here
		d.anchorX, d.anchorPiece = s.Cur.X, s.Pieces
		p.sx, p.sy = p.x, p.y
	}
	dx, dy := p.dx(), p.dy()
	if abs(dx) > 10 || abs(dy) > 10 {
		d.tapCand = false
	}
	// flick down: hard drop, once per gesture
	if p.vy > 1100 && dy > 46 && abs(dy) > 1.6*abs(dx) {
		s.HardDrop()
		p.consumed = true
		return false
	}
	// steer: the piece follows the finger in half-block steps
	if abs(dx) > 8 && abs(dx) > abs(dy)*0.6 {
		p.dragging = true
	}
	if p.dragging {
		step := s.Step()
		target := d.anchorX + int(dx/d.g/float64(step)+sign(dx)*0.5)*step
		for i := 0; i < 8 && s.Cur.X != target; i++ {
			dir := step
			if target < s.Cur.X {
				dir = -step
			}
			if !s.Shift(dir) {
				break
			}
		}
	}
	return dy > 34 && abs(dy) > 1.2*abs(dx) && p.vy < 1100
}

func sign(v float64) float64 {
	if v < 0 {
		return -1
	}
	return 1
}

// buttonControls: ◀ ▶ (hold to repeat), turn, soft, drop; mirrored for the
// left-handed layout.
func (d *dropPlay) buttonControls(dt float64) bool {
	a, s := d.a, d.sim
	x, w := d.btnRow()
	bw := w / 5
	slot := func(i int) float64 {
		if a.st.data.Settings.LeftHanded {
			i = 4 - i
		}
		return x + float64(i)*bw
	}
	y, h := d.ctrlY+12, 76.0
	dir := 0
	if a.in.pressing(slot(0), y, bw, h) != nil {
		dir = -1
	}
	if a.in.pressing(slot(1), y, bw, h) != nil {
		dir = 1
	}
	if dir != d.dasDir {
		d.dasDir, d.dasT = dir, 0
		if dir != 0 {
			s.Shift(dir * s.Step())
		}
	} else if dir != 0 {
		d.dasT += dt
		for d.dasT >= 0.16+1.0/40 {
			d.dasT -= 1.0 / 40
			s.Shift(dir * s.Step())
		}
	}
	if a.in.tapped(slot(2), y, bw, h) {
		s.Turn(1)
	}
	if a.in.tapped(slot(4), y, bw, h) {
		s.HardDrop()
	}
	return a.in.pressing(slot(3), y, bw, h) != nil
}

func (d *dropPlay) btnRow() (x, w float64) {
	sx, _, sw, _ := d.a.safe()
	return sx + 12, sw - 24
}

// ── events → feedback ────────────────────────────────────────────────────

var chainNames = [...]string{"", "CLEAR", "DOUBLE CHAIN", "TRIPLE CHAIN", "MEGA CHAIN"}

func (d *dropPlay) events() {
	a, s := d.a, d.sim
	g := d.g
	d.trickT -= core.Dt
	for e, ok := s.PollEvent(); ok; e, ok = s.PollEvent() {
		switch e.Kind {
		case core.EvMove:
			a.sfx(sfxMove, 0.35)
		case core.EvTurn:
			a.sfx(sfxTurn, 0.6)
			a.haptic(HapticLight)
		case core.EvLand, core.EvDrop:
			if e.Kind == core.EvDrop {
				a.sfx(sfxDrop, 1)
				a.haptic(HapticHeavy)
				a.fx.kick(5)
			} else {
				a.sfx(sfxLand, 0.8)
				a.haptic(HapticMedium)
			}
			col := fruit[e.Colour]
			for _, c := range core.Shapes[e.Piece][e.Rot] {
				bx := d.bx + float64(e.X+int(c[0])*s.B)*g
				by := d.by + float64(e.Y+(int(c[1])+1)*s.B)*g
				for i := 0; i < 5; i++ {
					a.fx.dust(bx+float64(s.B)*g*float64(i)/4, by, col, float32(g*0.9))
				}
			}
		case core.EvTrickle:
			if d.trickT <= 0 {
				d.trickT = 0.1
				a.sfx(sfxTrickle, 0.2+0.4*float32(min(1, float64(e.N)/float64(s.GW*2))))
			}
		case core.EvClear:
			d.onClear(e)
		case core.EvVanish:
			for i := 0; i < d.nsnap; i++ {
				p := d.snap[i]
				x, y := d.bx+(float64(p.x)+0.5)*g, d.by+(float64(p.y)+0.5)*g
				a.fx.dust(x, y, fruit[p.c], float32(g*1.1))
				if i%3 == 0 {
					a.fx.sparkle(x, y, fruit[p.c], float32(g*1.4))
				}
			}
			d.nsnap = 0
		case core.EvLevelUp:
			a.sfx(sfxLevel, 0.8)
			d.banner, d.bannerT = "LEVEL "+itoa(e.N), 0
		case core.EvGameOver:
			a.sfx(sfxOver, 1)
			a.haptic(HapticError)
		}
	}
}

func (d *dropPlay) onClear(e core.Event) {
	a, s := d.a, d.sim
	// snapshot a sample of the flashing grains for the burst when they go
	cl := s.Clearing()
	step := max(1, len(cl)/len(d.snap))
	d.nsnap = 0
	sumY := 0.0
	for i := 0; i < len(cl) && d.nsnap < len(d.snap); i += step {
		j := int(cl[i])
		x, y := j%s.GW, j/s.GW
		d.snap[d.nsnap] = struct {
			x, y int32
			c    uint8
		}{int32(x), int32(y), s.Sand[j] & core.ColourMask}
		d.nsnap++
		sumY += float64(y)
	}
	if s.Phase != core.Playing {
		return
	}
	ch := e.Chain
	if ch > 1 {
		a.sfx(sfxChain, 1)
	} else {
		a.sfx(sfxClear, 1)
	}
	if ch >= 3 {
		a.haptic(HapticSuccess)
	} else {
		a.haptic(HapticMedium)
	}
	a.fx.kick(float64(2 + 2*min(ch, 4)))
	name := "LEMON STORM ×" + itoa(ch)
	if ch < len(chainNames) {
		name = chainNames[ch]
	}
	cy := d.by + (sumY/float64(max(1, d.nsnap)))*d.g
	col := fruit[e.Colour]
	a.fx.say(name, d.bx+d.bw/2, cy-28, 26+3*float64(min(ch, 5)), col.lighten(0.3), 1.3, true)
	a.fx.say("+"+grouped(e.Points), d.bx+d.bw/2, cy+8, 20, textCol, 1.1, false)
	for i := 0; i < d.nsnap; i += 4 {
		p := d.snap[i]
		a.fx.sparkle(d.bx+(float64(p.x)+0.5)*d.g, d.by+(float64(p.y)+0.5)*d.g, col, float32(d.g*1.6))
	}
}

// ── draw ─────────────────────────────────────────────────────────────────

func (d *dropPlay) draw(c *canvas) {
	a, s := d.a, d.sim
	d.layout()
	ox, oy := a.fx.shakeOffset(a.t)
	bx, by := d.bx+ox, d.by+oy

	// board frame (shader): panel, inner glow, citrus edge; pulses on clears
	pulse := float32(0)
	if s.FlashT > 0 {
		pulse = float32(s.FlashT / core.FlashTime)
	}
	drawPanel(c, bx-6, by-6, d.bw+12, d.bh+12, 16, pulse, boardEdge)

	// sand
	d.tex.fill(s, a.t)
	glow := float32(1)
	if s.Phase == core.Over {
		glow = float32(1 - 0.45*clamp01(d.overT/1.0))
	}
	d.tex.draw(c, bx, by, d.g, a.t, glow)

	// ghost and piece
	if s.Phase == core.Playing {
		p := s.Cur
		block := float64(s.B) * d.g
		col := fruit[p.Colour]
		gy := s.DropY()
		drawGhost(c, p.Kind, p.Rot, col, bx+float64(p.X)*d.g, by+float64(gy)*d.g, block)
		fy := float64(p.Y)
		if s.Fits(p.Kind, p.Rot, p.X, p.Y+1) {
			fy += min(0.99, p.Fall+s.Alpha()*core.FallSpeed(s.Level)*float64(s.B)*core.Dt)
		}
		c.clip(bx, by-4*block, d.bw, d.bh+4*block)
		drawPiece(c, p.Kind, p.Rot, col, bx+float64(p.X)*d.g, by+fy*d.g, block, 1)
		c.unclip()
	}
	d.drawHUD(c)
	if a.st.data.Settings.Buttons {
		d.drawButtons(c)
	} else if s.Pieces < 3 && s.Phase == core.Playing {
		al := float32(0.55 * (1 - clamp01(float64(s.Pieces)/3)))
		c.text("drag ↔ steer   ·   tap turn   ·   flick ↓ drop", a.W/2, d.by+d.bh+20, 13, false, textCol, al, 1)
	}
	if d.bannerT < 1.6 && d.banner != "" {
		k := easeOutBack(d.bannerT / 0.3)
		al := float32(1 - clamp01((d.bannerT-1.1)/0.5))
		c.textScaled(d.banner, a.W/2, d.by+d.bh*0.3, 30, k, accent, al)
	}
	if s.Phase == core.Over {
		al := float32(clamp01(d.overT / 0.6))
		c.textScaled("GAME OVER", a.W/2, d.by+d.bh*0.42, 40, easeOutBack(d.overT/0.4), danger.lighten(0.2), al)
	}
	if a.paused {
		d.drawPause(c)
	}
}

func (d *dropPlay) drawHUD(c *canvas) {
	a, s := d.a, d.sim
	sx, _, sw, _ := a.safe()
	y := d.hudY + 14
	// score
	c.text("SCORE", sx+20, y+8, 12, true, dimText, 1, 0)
	c.text(grouped(d.score.value()), sx+20, y+34, 30, true, textCol, 1, 0)
	best := max(a.st.best(), s.Score)
	c.text("BEST "+grouped(best), sx+20, y+62, 12.5, false, dimText, 1, 0)
	// level & chain
	mid := sx + sw*0.52
	c.text("LEVEL", mid, y+8, 12, true, dimText, 1, 1)
	c.text(itoa(s.Level), mid, y+34, 26, true, accent, 1, 1)
	if s.Combo > 1 {
		c.text("CHAIN ×"+itoa(s.Combo), mid, y+62, 13, true, accent2, 1, 1)
	} else {
		c.text(itoa(s.Clears)+" CLEARS", mid, y+62, 11.5, false, dimText, 1, 1)
	}
	// next
	nx := sx + sw - 128
	c.round(nx, y, 62, 62, 14, cardBg, 0.9)
	c.text("NEXT", nx+31, y-6+0, 10, true, dimText, 1, 1)
	n := s.Next
	nb := 11.0
	minX, maxX, minY, maxY := 4, 0, 4, 0
	for _, cell := range core.Shapes[n.Kind][0] {
		minX, maxX = min(minX, int(cell[0])), max(maxX, int(cell[0]))
		minY, maxY = min(minY, int(cell[1])), max(maxY, int(cell[1]))
	}
	pw, ph := float64(maxX-minX+1)*nb, float64(maxY-minY+1)*nb
	drawPiece(c, n.Kind, 0, fruit[n.Colour], nx+31-pw/2-float64(minX)*nb, y+31-ph/2-float64(minY)*nb, nb, 1)
	// pause
	a.iconButton(c, d.pauseX(), y, 44, pauseGlyph)
}

func (d *dropPlay) drawButtons(c *canvas) {
	a := d.a
	x, w := d.btnRow()
	bw := w / 5
	labels := [5]string{"◀", "▶", "⟳", "▼", "⤓"}
	for i := 0; i < 5; i++ {
		j := i
		if a.st.data.Settings.LeftHanded {
			j = 4 - i
		}
		bx := x + float64(j)*bw
		y := d.ctrlY + 12
		press := a.in.pressing(bx, y, bw, 76) != nil
		k := 1.0
		if press {
			k = 0.94
		}
		cx, cy := bx+bw/2, y+38
		r := min(bw, 76) * 0.42 * k
		c.circle(cx, cy+3, r, rgb{}.rgba(0.3))
		col := cardBg.lighten(0.08)
		if i == 4 {
			col = accent2
		}
		c.circle(cx, cy, r, col.rgba(1))
		tc := textCol
		if i == 4 {
			tc = hex(0x3a2a00)
		}
		c.text(labels[i], cx, cy+1, r*0.85, true, tc, 1, 1)
	}
}

// ── pause ────────────────────────────────────────────────────────────────

func (d *dropPlay) pauseRects() (x, y, w float64) {
	a := d.a
	w = min(a.W-64, 320)
	return (a.W - w) / 2, a.H*0.5 - 130, w
}

func (d *dropPlay) updatePause() {
	a := d.a
	x, y, w := d.pauseRects()
	switch {
	case a.button(nil, "RESUME", x+24, y+66, w-48, 54, btnPrimary) || a.keys.just(ebiten.KeyP) || a.keys.just(ebiten.KeyEscape):
		a.paused = false
	case a.button(nil, "RESTART", x+24, y+130, w-48, 50, btnSecondary):
		a.startDrop()
	case a.button(nil, "SETTINGS", x+24, y+190, w-48, 50, btnSecondary):
		a.settingsRet = scrDrop
		a.go_(scrSettings)
	case a.button(nil, "MAIN MENU", x+24, y+250, w-48, 50, btnGhost):
		a.paused = false
		a.go_(scrMenu)
	}
}

func (d *dropPlay) drawPause(c *canvas) {
	a := d.a
	c.rect(0, 0, a.W, a.H, bgBottom.rgba(0.6))
	x, y, w := d.pauseRects()
	c.card(x, y, w, 320)
	c.text("PAUSED", x+w/2, y+34, 24, true, accent, 1, 1)
	a.button(c, "RESUME", x+24, y+66, w-48, 54, btnPrimary)
	a.button(c, "RESTART", x+24, y+130, w-48, 50, btnSecondary)
	a.button(c, "SETTINGS", x+24, y+190, w-48, 50, btnSecondary)
	a.button(c, "MAIN MENU", x+24, y+250, w-48, 50, btnGhost)
}

// drawPanel is the board frame shader around (x, y, w, h) dp.
func drawPanel(c *canvas, x, y, w, h, r float64, pulse float32, edge rgb) {
	pad := 28.0
	if c.panelUni == nil {
		c.panelV = [4][]float32{make([]float32, 4), make([]float32, 1), make([]float32, 1), make([]float32, 3)}
		c.panelUni = map[string]any{"Rect": c.panelV[0], "Radius": c.panelV[1], "Pulse": c.panelV[2], "Edge": c.panelV[3]}
	}
	v := &c.panelV
	v[0][0], v[0][1], v[0][2], v[0][3] = c.px(x), c.px(y), c.px(w), c.px(h)
	v[1][0], v[2][0] = c.px(r), pulse
	v[3][0], v[3][1], v[3][2] = edge.r, edge.g, edge.b
	op := &ebiten.DrawRectShaderOptions{Uniforms: c.panelUni}
	op.GeoM.Translate(float64(c.px(x-pad)), float64(c.px(y-pad)))
	c.dst.DrawRectShader(int(c.px(w+2*pad)), int(c.px(h+2*pad)), panelShader, op)
}
