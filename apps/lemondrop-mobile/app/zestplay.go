package app

import (
	"math/bits"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/thebanri/limoni/apps/lemondrop-mobile/blocks"
	"github.com/thebanri/limoni/apps/lemondrop-mobile/gen"
)

var emptyCell = hex(0x1d2130)

// zestPlay is the drag-and-place mode.
type zestPlay struct {
	a    *App
	g    *blocks.Game
	prof gen.ZestProfile

	bx, by, cell float64 // board, dp
	trayY, slotW float64
	hudY         float64

	score counter
	start float64

	drag   int // tray slot being dragged, -1 none
	dp     *pointer
	px, py float64 // dragged piece top-left, dp
	snapR  int
	snapC  int
	valid  bool
	back   [3]float64 // return animation per slot (1 → 0)
	appear [3]float64
	overT  float64
	clean  float64
}

func (a *App) startZest() {
	p := gen.ZestStandard
	switch a.st.data.Settings.ZestProfile {
	case gen.ZestRelaxed.Name:
		p = gen.ZestRelaxed
	case gen.ZestChallenge.Name:
		p = gen.ZestChallenge
	}
	z := &zestPlay{a: a, prof: p, drag: -1, start: a.t}
	z.g = blocks.New(a.nextSeed(), &gen.Zest{P: p})
	z.g.Events()
	for i := range z.appear {
		z.appear[i] = 1
	}
	a.zest = z
	a.paused = false
	a.go_(scrZest)
	a.sfx(sfxStart, 0.8)
}

func (z *zestPlay) layout() {
	a := z.a
	sx, sy, sw, sh := a.safe()
	const hud = 92.0
	tray := 150.0
	side := min(sw-32, sh-hud-tray-24)
	z.cell = side / blocks.N
	z.bx = sx + (sw-side)/2
	z.by = sy + hud + 12 + max(0, (sh-hud-tray-24-side)/2)
	z.trayY = z.by + side + 26
	z.slotW = sw / 3
	z.hudY = sy
}

func (z *zestPlay) slotRect(i int) (x, y, w, h float64) {
	sx, _, _, _ := z.a.safe()
	return sx + float64(i)*z.slotW, z.trayY, z.slotW, 120
}

// ── update ───────────────────────────────────────────────────────────────

func (z *zestPlay) update(dt float64) {
	a, g := z.a, z.g
	z.layout()
	z.score.set(g.Score)
	z.score.update(dt)
	for i := range z.back {
		z.back[i] = max(0, z.back[i]-dt*5)
		z.appear[i] = max(0, z.appear[i]-dt*4)
	}
	z.clean = max(0, z.clean-dt)
	if a.paused {
		z.updatePause()
		return
	}
	if a.iconButton(nil, z.pauseX(), z.hudY+14, 44, nil) || a.keys.just(ebiten.KeyP) || a.keys.just(ebiten.KeyEscape) {
		a.paused = true
		a.sfx(sfxPause, 0.7)
		return
	}
	if g.Over {
		z.overT += dt
		if z.overT > 1.3 {
			a.finishZest(z)
		}
		return
	}
	z.handleDrag()
	z.events()
}

func (z *zestPlay) pauseX() float64 { sx, _, sw, _ := z.a.safe(); return sx + sw - 58 }

func (z *zestPlay) handleDrag() {
	a, g := z.a, z.g
	in := &a.in
	if z.drag < 0 {
		for i := range in.ptrs {
			p := &in.ptrs[i]
			if !p.down || p.consumed {
				continue
			}
			for s := 0; s < 3; s++ {
				x, y, w, h := z.slotRect(s)
				if !g.Tray[s].Used && inside(p.x, p.y, x, y-20, w, h+30) {
					z.drag, z.dp = s, p
					a.sfx(sfxTap, 0.5)
					a.haptic(HapticLight)
				}
			}
		}
		return
	}
	p := z.dp
	sh := blocks.Shapes[g.Tray[z.drag].Shape]
	w, h := float64(sh.W)*z.cell, float64(sh.H)*z.cell
	// the piece rides above the finger so the finger never hides it
	z.px, z.py = p.x-w/2, p.y-h-z.cell*1.1
	z.snapC = int((z.px-z.bx)/z.cell + 0.5)
	z.snapR = int((z.py-z.by)/z.cell + 0.5)
	z.valid = g.CanPlace(z.drag, z.snapR, z.snapC)
	if p.up || !p.active {
		if z.valid {
			g.Place(z.drag, z.snapR, z.snapC)
		} else {
			z.back[z.drag] = 1
		}
		z.drag, z.dp = -1, nil
	}
}

func (z *zestPlay) events() {
	a := z.a
	for _, e := range z.g.Events() {
		switch e.Kind {
		case blocks.EvPlaced:
			a.sfx(sfxPlace, 0.8)
			a.haptic(HapticLight)
			for m := e.Mask; m != 0; m &= m - 1 {
				i := bits.TrailingZeros64(m)
				x, y := z.cellXY(i)
				a.fx.sparkle(x+z.cell/2, y+z.cell/2, accent, float32(z.cell*0.12))
			}
		case blocks.EvCleared:
			if e.Combo > 1 || e.Lines > 1 {
				a.sfx(sfxChain, 1)
			} else {
				a.sfx(sfxClear, 1)
			}
			if e.Lines >= 3 || e.Combo >= 3 {
				a.haptic(HapticSuccess)
			} else {
				a.haptic(HapticMedium)
			}
			a.fx.kick(float64(2 + e.Lines))
			for m := e.Mask; m != 0; m &= m - 1 {
				i := bits.TrailingZeros64(m)
				x, y := z.cellXY(i)
				col := fruit[max(1, e.Colours[i])]
				for k := 0; k < 6; k++ {
					a.fx.dust(x+z.cell*(0.2+0.6*float64(k%3)/2), y+z.cell*(0.3+0.4*float64(k/3)), col, float32(z.cell*0.13))
				}
				a.fx.sparkle(x+z.cell/2, y+z.cell/2, col, float32(z.cell*0.2))
			}
			cx, cy := z.bx+z.cell*4, z.by+z.cell*3.2
			label := ""
			switch {
			case e.Lines >= 4:
				label = "LEMON STORM"
			case e.Lines == 3:
				label = "TRIPLE SQUEEZE"
			case e.Lines == 2:
				label = "DOUBLE SQUEEZE"
			}
			if e.Combo > 1 {
				if label != "" {
					label += "  "
				}
				label += "COMBO ×" + itoa(e.Combo)
			}
			if label != "" {
				a.fx.say(label, cx, cy, 26, accent, 1.3, true)
			}
			if e.Zest > 0 {
				a.fx.say("ZEST LINE ×2", cx, cy+34, 20, fruit[2].lighten(0.3), 1.2, true)
			}
			a.fx.say("+"+grouped(e.Points), cx, cy+62, 20, textCol, 1.0, false)
		case blocks.EvComboLost:
			a.fx.say("combo over", z.bx+z.cell*4, z.by-10, 14, dimText, 0.9, false)
		case blocks.EvClean:
			z.clean = 1.6
			a.sfx(sfxHigh, 0.9)
			a.haptic(HapticSuccess)
			a.fx.say("SQUEEZED CLEAN  +300", z.bx+z.cell*4, z.by+z.cell*4, 28, accent, 1.6, true)
		case blocks.EvDeal:
			for i := range z.appear {
				z.appear[i] = 1
			}
		case blocks.EvGameOver:
			a.sfx(sfxOver, 1)
			a.haptic(HapticError)
		}
	}
}

func (z *zestPlay) cellXY(i int) (float64, float64) {
	return z.bx + float64(i%blocks.N)*z.cell, z.by + float64(i/blocks.N)*z.cell
}

// ── draw ─────────────────────────────────────────────────────────────────

func drawCell(c *canvas, x, y, s float64, col rgb, a float32) {
	c.blit(c.sprite(sprCell, col, s), x, y, a)
}

func (z *zestPlay) draw(c *canvas) {
	a, g := z.a, z.g
	z.layout()
	ox, oy := a.fx.shakeOffset(a.t)
	bx, by := z.bx+ox, z.by+oy
	side := z.cell * blocks.N
	drawPanel(c, bx-8, by-8, side+16, side+16, 18, float32(z.clean*0.6), boardEdge)

	// preview: which lines this placement would clear
	var prevMask, clearMask uint64
	if z.drag >= 0 && z.valid {
		sh := blocks.Shapes[g.Tray[z.drag].Shape]
		prevMask = sh.At(z.snapR, z.snapC)
		rows, cols := blocks.Full(g.Occ | prevMask)
		clearMask = blocks.ClearMask(rows, cols)
	}
	for i := 0; i < blocks.N*blocks.N; i++ {
		x, y := bx+float64(i%blocks.N)*z.cell, by+float64(i/blocks.N)*z.cell
		if g.Occ>>i&1 != 0 {
			col := fruit[max(1, g.Colour[i])]
			if clearMask>>i&1 != 0 {
				col = col.lighten(0.35 + 0.15*float32(sin(a.t*14)))
			}
			drawCell(c, x, y, z.cell, col, 1)
			continue
		}
		c.blit(c.sprite(sprEmpty, emptyCell, z.cell), x, y, 1)
		if prevMask>>i&1 != 0 {
			col := fruit[g.Tray[z.drag].Colour]
			drawCell(c, x, y, z.cell, col, 0.35)
		}
	}
	// tray
	for s := 0; s < 3; s++ {
		sl := g.Tray[s]
		if sl.Used || s == z.drag {
			continue
		}
		sh := blocks.Shapes[sl.Shape]
		x, y, w, h := z.slotRect(s)
		k := 0.55 * (1 - 0.6*z.appear[s])
		if z.back[s] > 0 {
			k *= 1 + 0.25*z.back[s]
		}
		cs := z.cell * k
		px, py := x+(w-float64(sh.W)*cs)/2, y+(h-float64(sh.H)*cs)/2
		fits := blocks.AnyFit(g.Occ, sh)
		al := float32(1 - z.appear[s])
		if !fits {
			al *= 0.35
		}
		for r := 0; r < sh.H; r++ {
			for cc := 0; cc < sh.W; cc++ {
				if sh.Has(r, cc) {
					drawCell(c, px+float64(cc)*cs, py+float64(r)*cs, cs, fruit[sl.Colour], al)
				}
			}
		}
	}
	// the dragged piece
	if z.drag >= 0 {
		sl := g.Tray[z.drag]
		sh := blocks.Shapes[sl.Shape]
		col := fruit[sl.Colour]
		if !z.valid {
			col = col.mix(danger, 0.55)
		}
		for r := 0; r < sh.H; r++ {
			for cc := 0; cc < sh.W; cc++ {
				if sh.Has(r, cc) {
					drawCell(c, z.px+float64(cc)*z.cell, z.py+float64(r)*z.cell, z.cell, col, 0.95)
				}
			}
		}
	}
	z.drawHUD(c)
	if g.Over {
		c.textScaled("NO ROOM LEFT", a.W/2, z.by+side/2, 34, easeOutBack(z.overT/0.4), danger.lighten(0.2), float32(clamp01(z.overT/0.6)))
	}
	if a.paused {
		z.drawPause(c)
	}
}

func (z *zestPlay) drawHUD(c *canvas) {
	a, g := z.a, z.g
	sx, _, sw, _ := a.safe()
	y := z.hudY + 14
	c.text("SCORE", sx+20, y+8, 12, true, dimText, 1, 0)
	c.text(grouped(z.score.value()), sx+20, y+34, 30, true, textCol, 1, 0)
	c.text("BEST "+grouped(max(a.st.best(ModeZest), g.Score)), sx+20, y+62, 12.5, false, dimText, 1, 0)
	mid := sx + sw*0.55
	c.text("COMBO", mid, y+8, 12, true, dimText, 1, 1)
	cv := "–"
	if g.Combo > 0 {
		cv = "×" + itoa(g.Combo)
	}
	c.text(cv, mid, y+34, 26, true, accent, 1, 1)
	if g.Combo > 0 {
		left := blocks.MissMax - g.Miss
		c.text(itoa(left)+" moves to keep it", mid, y+62, 11.5, false, dimText, 1, 1)
	} else {
		c.text("ZEST · "+z.prof.Name, mid, y+62, 11.5, false, dimText, 1, 1)
	}
	a.iconButton(c, z.pauseX(), y, 44, pauseGlyph)
}

// ── pause ────────────────────────────────────────────────────────────────

func (z *zestPlay) updatePause() {
	a := z.a
	x, y, w := (&dropPlay{a: a}).pauseRects()
	switch {
	case a.button(nil, "RESUME", x+24, y+66, w-48, 54, btnPrimary) || a.keys.just(ebiten.KeyP) || a.keys.just(ebiten.KeyEscape):
		a.paused = false
	case a.button(nil, "RESTART", x+24, y+130, w-48, 50, btnSecondary):
		a.startZest()
	case a.button(nil, "SETTINGS", x+24, y+190, w-48, 50, btnSecondary):
		a.settingsRet = scrZest
		a.go_(scrSettings)
	case a.button(nil, "MAIN MENU", x+24, y+250, w-48, 50, btnGhost):
		a.paused = false
		a.go_(scrMenu)
	}
}

func (z *zestPlay) drawPause(c *canvas) { (&dropPlay{a: z.a}).drawPause(c) }
