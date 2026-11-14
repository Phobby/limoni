// Package app is Lemon Drop Mobile's Ebitengine frontend: screens, layout,
// touch input, GPU rendering, particles, sound and haptics. The rules live in
// core (falling sand) and gen (piece dealing); this package only reads them
// and turns their events into feedback.
package app

import (
	"time"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/thebanri/limoni/apps/lemondrop-mobile/core"
)

type screen int

const (
	scrSplash screen = iota
	scrMenu
	scrDrop
	scrSettings
	scrHelp
	scrBoard
	scrOver
	scrName
)

// Options configure an App.
type Options struct {
	Seed    uint64 // 0: from the clock
	NoSound bool
	StartAt string  // debug: "menu", "drop", "settings", … (screenshots)
	Speed   float64 // debug: time multiplier (screenshots); 0 means 1
}

// App is the whole game; it implements ebiten.Game.
type App struct {
	plat Platform
	opt  Options
	st   *store
	snd  *sound
	fx   *fxSystem
	in   input
	keys keys

	W, H  float64 // screen size in dp
	scale float64 // pixels per dp
	t     float64 // seconds since start
	last  time.Time
	frame int

	scr, prev   screen
	transT      float64
	settingsRet screen
	paused      bool

	drop    *dropPlay
	attract *core.Sim
	atex    *sandTex

	res     result
	typing  string
	nameRet screen
	seedN   uint64

	uni        map[string]any // reused shader uniforms
	wasFocused bool
	setScroll  float64
	bgV        [4][]float32
	bgOp       ebiten.DrawRectShaderOptions
	cv         canvas
}

// New makes the game for a platform.
func New(p Platform, o Options) *App {
	loadShaders()
	a := &App{plat: p, opt: o, fx: newFX(), scr: scrSplash, last: time.Now()}
	a.st = newStore(p.DataDir())
	a.fx.reduce = a.st.data.Settings.ReduceMove
	if !o.NoSound {
		a.snd = newSound()
	}
	a.applyVolumes()
	a.seedN = o.Seed
	if a.seedN == 0 {
		a.seedN = uint64(time.Now().UnixNano())
	}
	a.attract = core.NewSim(a.nextSeed(), core.MaxB)
	a.atex = newSandTex()
	a.uni = map[string]any{}
	switch o.StartAt {
	case "menu":
		a.scr = scrMenu
	case "drop":
		a.startDrop()
	case "settings":
		a.scr = scrSettings
	case "help":
		a.scr = scrHelp
	case "board":
		a.scr = scrBoard
	}
	return a
}

func (a *App) nextSeed() uint64 {
	a.seedN = a.seedN*6364136223846793005 + 1442695040888963407
	return a.seedN | 1
}

func (a *App) applyVolumes() {
	if a.snd == nil {
		return
	}
	s := a.st.data.Settings
	if s.Muted {
		a.snd.m.setVolumes(0, 0)
		return
	}
	a.snd.m.setVolumes(float32(s.SfxVol), float32(s.MusicVol))
}

func (a *App) sfx(e sfx, vol float32) {
	if !a.st.data.Settings.Muted {
		a.snd.play(e, vol, 0)
	}
}

func (a *App) haptic(h Haptic) {
	if a.st.data.Settings.Haptics {
		a.plat.Haptic(h)
	}
}

func (a *App) go_(s screen) {
	a.prev, a.scr, a.transT = a.scr, s, 0
	a.in.consumeAll()
}

// TextEntered delivers a native text prompt's answer (the player's name).
func (a *App) TextEntered(s string) {
	if n := cleanName(s); n != "" {
		a.st.data.Name = n
		a.st.save()
	}
}

// Suspend is called by the shells when the app goes to the background.
func (a *App) Suspend() {
	if a.scr == scrDrop {
		a.paused = true
	}
	a.st.save()
}

// ── ebiten.Game ──────────────────────────────────────────────────────────

func (a *App) Layout(w, h int) (int, int) { return w, h } // LayoutF is used

func (a *App) LayoutF(w, h float64) (float64, float64) {
	s := ebiten.Monitor().DeviceScaleFactor()
	if s <= 0 {
		s = 1
	}
	a.W, a.H, a.scale = w, h, s
	return w * s, h * s
}

func (a *App) Update() error {
	now := time.Now()
	dt := min(now.Sub(a.last).Seconds(), 0.1)
	a.last = now
	if a.frame < 2 {
		dt = 1.0 / 60
	}
	if a.opt.Speed > 0 {
		dt = a.opt.Speed / 60 // fixed, for reproducible screenshots
	}
	a.frame++
	a.t += dt
	a.transT += dt
	a.in.update(a.t, a.scale, dt)
	a.fx.update(dt)
	// Pause when the app loses focus (backgrounded), not because it started unfocused.
	focused := ebiten.IsFocused()
	if a.opt.Speed == 0 && a.wasFocused && !focused && a.scr == scrDrop { // not in scripted screenshot runs
		a.paused = true
		a.st.save()
	}
	a.wasFocused = focused
	switch a.scr {
	case scrSplash:
		if a.t > 1.6 || a.in.anyUp() {
			a.go_(scrMenu)
		}
		a.attract.Update(dt)
	case scrMenu, scrSettings, scrHelp, scrBoard, scrName:
		a.attract.Update(dt)
		a.updateMenus()
	case scrDrop:
		a.drop.update(dt)
	case scrOver:
		a.updateOver(dt)
	}
	return nil
}

func (a *App) Draw(dst *ebiten.Image) {
	a.cv.dst, a.cv.s = dst, float32(a.scale)
	c := &a.cv
	a.drawBackground(dst)
	switch a.scr {
	case scrSplash:
		a.drawSplash(c)
	case scrDrop:
		a.drop.draw(c)
	case scrOver:
		a.drawOver(c)
	default:
		a.drawMenus(c)
	}
	ox, oy := a.fx.shakeOffset(a.t)
	a.fx.draw(c, ox, oy)
	a.fx.drawTexts(c)
	if a.transT < 0.25 && a.scr != scrSplash {
		a.cv.rect(0, 0, a.W, a.H, bgBottom.rgba(float32(1-easeOutCubic(a.transT/0.25))*0.85))
	}
}

// safe returns the safe rectangle in dp.
func (a *App) safe() (x, y, w, h float64) {
	t, l, b, r := a.plat.SafeArea()
	return l, t, a.W - l - r, a.H - t - b
}

func (a *App) drawBackground(dst *ebiten.Image) {
	w, h := float64(dst.Bounds().Dx()), float64(dst.Bounds().Dy())
	lx, ly, lr := w*0.5, h*0.56, min(w, h)*0.44
	if a.scr == scrDrop && a.drop != nil {
		lx, ly, lr = (a.drop.bx+a.drop.bw/2)*a.scale, (a.drop.by+a.drop.bh*0.56)*a.scale, a.drop.bw*0.44*a.scale
	}
	amt := float32(0.10)
	if a.scr == scrSplash || a.scr == scrMenu {
		amt = 0.16
	}
	if len(a.uni) == 0 {
		a.bgV = [4][]float32{make([]float32, 1), make([]float32, 2), make([]float32, 3), make([]float32, 1)}
		a.uni["Time"], a.uni["Size"], a.uni["Lemon"], a.uni["Amount"] = a.bgV[0], a.bgV[1], a.bgV[2], a.bgV[3]
		a.uni["Top"] = []float32{bgTop.r, bgTop.g, bgTop.b}
		a.uni["Bottom"] = []float32{bgBottom.r, bgBottom.g, bgBottom.b}
	}
	a.bgV[0][0] = float32(a.t)
	a.bgV[1][0], a.bgV[1][1] = float32(w), float32(h)
	a.bgV[2][0], a.bgV[2][1], a.bgV[2][2] = float32(lx), float32(ly), float32(lr)
	a.bgV[3][0] = amt
	a.bgOp.Uniforms = a.uni
	dst.DrawRectShader(int(w), int(h), bgShader, &a.bgOp)
}

// ── splash ───────────────────────────────────────────────────────────────

func (a *App) drawSplash(c *canvas) {
	k := easeOutBack(a.t / 0.9)
	al := float32(clamp01(a.t / 0.5))
	cx, cy := a.W/2, a.H*0.42
	c.circle(cx, cy, 70*k, rindCol.rgba(al))
	c.circle(cx, cy, 63*k, pithCol.rgba(al))
	c.circle(cx, cy, 56*k, fleshCol.rgba(al))
	for i := 0; i < 10; i++ {
		ang := float64(i)/10*6.2831853 + a.t*0.4
		x2, y2 := cx+56*k*cos(ang), cy+56*k*sin(ang)
		c.line(cx, cy, x2, y2, 2.4, pithCol, al)
	}
	c.circle(cx, cy, 7*k, pithCol.rgba(al))
	ta := float32(clamp01((a.t - 0.4) / 0.5))
	c.text("LEMON DROP", cx, cy+112, 38, true, accent, ta, 1)
	c.text("falling blocks · living sand", cx, cy+148, 15, false, dimText, ta, 1)
}
