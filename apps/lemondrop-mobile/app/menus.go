package app

import (
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
)

// Menus are immediate-mode: one function per screen lays out, hit-tests
// (c == nil, from Update) and draws (from Draw), so taps and pixels agree.

func (a *App) updateMenus() {
	switch a.scr {
	case scrMenu:
		a.uiMenu(nil)
	case scrSettings:
		a.uiSettings(nil)
	case scrHelp:
		a.uiHelp(nil)
	case scrBoard:
		a.uiBoard(nil)
	case scrName:
		a.uiName(nil)
	}
	if a.keys.just(ebiten.KeyEscape) && a.scr != scrMenu && a.scr != scrName {
		a.back()
	}
}

func (a *App) drawMenus(c *canvas) {
	if a.scr == scrMenu {
		// full width at the bottom, faint: a bed of sand under the menu
		g := a.W / float64(a.attract.GW)
		h := g * float64(a.attract.GH)
		a.atex.fill(a.attract, a.t)
		a.atex.draw(c, 0, a.H-h, g, a.t, 0.38)
	}
	switch a.scr {
	case scrMenu:
		a.uiMenu(c)
	case scrSettings:
		a.uiSettings(c)
	case scrHelp:
		a.uiHelp(c)
	case scrBoard:
		a.uiBoard(c)
	case scrName:
		a.uiName(c)
	}
}

func (a *App) back() {
	switch a.scr {
	case scrSettings:
		if a.settingsRet == scrDrop {
			a.go_(scrDrop)
			a.settingsRet = scrMenu
			return
		}
	case scrName:
		a.go_(a.nameRet)
		return
	}
	a.go_(scrMenu)
}

// header draws a screen title with a back button; reports a back tap.
func (a *App) header(c *canvas, title string) bool {
	sx, sy, sw, _ := a.safe()
	if c == nil {
		return a.iconButton(nil, sx+14, sy+14, 44, nil)
	}
	a.iconButton(c, sx+14, sy+14, 44, backGlyph)
	c.text(title, sx+sw/2, sy+36, 22, true, textCol, 1, 1)
	return false
}

// ── main menu ────────────────────────────────────────────────────────────

func (a *App) uiMenu(c *canvas) {
	_, sy, sw, _ := a.safe()
	w := min(sw-48, 340)
	x := (a.W - w) / 2
	y := sy + a.H*0.30
	if c != nil {
		pop := easeOutBack((a.transT + 0.2) / 0.6)
		c.textScaled("LEMON", a.W/2, sy+a.H*0.11, 52, pop, accent, 1)
		c.textScaled("DROP", a.W/2, sy+a.H*0.11+54, 52, pop, accent2, 1)
		name := a.st.data.Name
		if name == "" {
			name = "set your name"
		}
		c.round(a.W/2-90, y-58, 180, 34, 17, cardBg, 0.85)
		c.text("● "+name, a.W/2, y-41, 14, false, textCol, 1, 1)
		if best := a.st.best(); best > 0 {
			c.text("BEST "+grouped(best), a.W/2, y-84, 13, true, dimText, 1, 1)
		}
	} else if a.in.tapped(a.W/2-90, y-58, 180, 34) {
		a.askName(scrMenu)
	}
	if a.button(c, "PLAY", x, y, w, 62, btnPrimary) {
		a.startDrop()
	}
	if a.button(c, "SCORES", x, y+76, w, 52, btnSecondary) {
		a.go_(scrBoard)
	}
	if a.button(c, "HOW TO PLAY", x, y+140, w, 52, btnSecondary) {
		a.go_(scrHelp)
	}
	if a.button(c, "SETTINGS", x, y+204, w, 52, btnSecondary) {
		a.settingsRet = scrMenu
		a.go_(scrSettings)
	}
}

// ── settings ─────────────────────────────────────────────────────────────

func (a *App) uiSettings(c *canvas) {
	s := &a.st.data.Settings
	if a.header(c, "SETTINGS") {
		a.st.save()
		a.back()
		return
	}
	_, sy, sw, sh := a.safe()
	w := min(sw-24, 460)
	x := (a.W - w) / 2
	const content = 466.0
	view := sh - 84
	a.scrollSettings(view, content)
	y := sy + 74 - a.setScroll
	if c != nil {
		c.clip(0, sy+66, a.W, view+18)
		defer c.unclip()
		c.card(x, y, w, content)
	}
	changed := false
	set := func(dst *float64, v float64) {
		if c == nil && v != *dst {
			*dst, changed = v, true
		}
	}
	setB := func(dst *bool, v bool) {
		if c == nil && v != *dst {
			*dst, changed = v, true
		}
	}
	set(&s.MusicVol, a.slider(c, "Music", x, y+2, w, s.MusicVol))
	set(&s.SfxVol, a.slider(c, "Sound effects", x, y+58, w, s.SfxVol))
	setB(&s.Muted, a.toggle(c, "Mute everything", x, y+114, w, s.Muted))
	setB(&s.Haptics, a.toggle(c, "Haptics", x, y+160, w, s.Haptics))
	setB(&s.ReduceMove, a.toggle(c, "Reduce motion", x, y+206, w, s.ReduceMove))
	ctl := 0
	if s.Buttons {
		ctl = 1
	}
	if v := a.segmented(c, "Controls", x, y+252, w, []string{"Gestures", "Buttons"}, ctl); c == nil && v != ctl {
		s.Buttons, changed = v == 1, true
	}
	hand := 0
	if s.LeftHanded {
		hand = 1
	}
	if v := a.segmented(c, "Button layout", x, y+330, w, []string{"Right hand", "Left hand"}, hand); c == nil && v != hand {
		s.LeftHanded, changed = v == 1, true
	}
	name := a.st.data.Name
	if name == "" {
		name = "not set"
	}
	if c != nil {
		c.text("Player name", x+16, y+430, 17, false, textCol, 1, 0)
		c.text(name+"  ›", x+w-16, y+430, 16, true, accent, 1, 2)
	} else if a.in.tapped(x, y+406, w, 50) {
		a.askName(scrSettings)
	}
	if changed {
		a.fx.reduce = s.ReduceMove
		a.applyVolumes()
		a.st.save()
	}
}

// scrollSettings drags the settings list when it is taller than the view.
func (a *App) scrollSettings(view, content float64) {
	maxOff := max(0, content-view+10)
	for i := range a.in.ptrs {
		p := &a.in.ptrs[i]
		if p.active && !p.up && abs(p.dy()) > 12 && abs(p.dy()) > abs(p.dx()) && maxOff > 0 {
			a.setScroll -= p.y - p.py
			p.consumed = true
		}
	}
	a.setScroll = max(0, min(maxOff, a.setScroll))
}

// ── how to play ──────────────────────────────────────────────────────────

const helpText = `Pieces fall like in any falling-block game, but when one lands it crumbles into sand of its colour and runs down the pile.

A run of ONE colour that reaches from the left wall to the right wall flashes and clears. Grains touching at a corner still connect.

Whatever rested on it falls — and may make another clear: a CHAIN. Chains multiply your points.

Gestures: drag sideways to steer · tap the left or right half to turn · flick down to drop · drag down and hold to fall faster.

Score: base × level × chain. Base is the cleared grains, ten points a block's worth. The level rises every four clears and pieces fall faster. Dropping a piece scores 2 a row.`

func (a *App) uiHelp(c *canvas) {
	if a.header(c, "HOW TO PLAY") {
		a.back()
		return
	}
	if c == nil {
		return
	}
	_, sy, sw, sh := a.safe()
	w := min(sw-24, 520)
	x := (a.W - w) / 2
	y := sy + 74
	c.card(x, y, w, min(sh-90, 600))
	c.text("FALLING SAND", x+20, y+28, 18, true, accent, 1, 0)
	const size = 14.5
	ly := y + 62
	for _, para := range strings.Split(helpText, "\n\n") {
		for _, line := range wrap(para, w-40, size, c.s) {
			c.text(line, x+20, ly, size, false, textCol, 1, 0)
			ly += size * 1.45
		}
		ly += size * 0.7
	}
}

// wrap breaks s into lines no wider than w dp.
func wrap(s string, w, size float64, scale float32) []string {
	var out []string
	line := ""
	for _, word := range strings.Fields(s) {
		try := word
		if line != "" {
			try = line + " " + word
		}
		if line != "" && textWidth(try, size, false, scale) > w {
			out = append(out, line)
			line = word
			continue
		}
		line = try
	}
	if line != "" {
		out = append(out, line)
	}
	return out
}

// ── scores (this device) ─────────────────────────────────────────────────

func (a *App) uiBoard(c *canvas) {
	if a.header(c, "SCORES") {
		a.back()
		return
	}
	if c == nil {
		return
	}
	_, sy, sw, sh := a.safe()
	w := min(sw-24, 520)
	x := (a.W - w) / 2
	y := sy + 74
	h := min(sh-90, 680)
	c.card(x, y, w, h)
	rows := a.st.board()
	if len(rows) == 0 {
		c.text("No runs yet — play one!", x+w/2, y+h/2, 16, false, dimText, 1, 1)
		return
	}
	rh := min(30.0, (h-40)/float64(boardSize))
	me := a.st.data.Name
	for i, e := range rows {
		if i >= boardSize {
			break
		}
		ry := y + 30 + float64(i)*rh
		col := textCol
		if e.Name == me && me != "" {
			c.round(x+10, ry-rh/2+2, w-20, rh-4, 8, accent2, 0.18)
			col = accent
		}
		c.text(itoa(i+1)+".", x+40, ry, 14, true, dimText, 1, 2)
		c.text(e.Name, x+52, ry, 14.5, false, col, 1, 0)
		c.text(grouped(e.Score), x+w-20, ry, 14.5, true, col, 1, 2)
	}
}

// ── name entry (desktop fallback; phones use a native prompt) ────────────

func (a *App) askName(ret screen) {
	if a.plat.RequestText("Your name for the scores", a.st.data.Name) {
		return
	}
	a.typing, a.nameRet = a.st.data.Name, ret
	a.go_(scrName)
}

var chars []rune

func (a *App) uiName(c *canvas) {
	_, sy, sw, _ := a.safe()
	if c == nil {
		chars = ebiten.AppendInputChars(chars[:0])
		for _, r := range chars {
			if len([]rune(a.typing)) < 12 {
				a.typing += string(r)
			}
		}
		if a.keys.repeat(ebiten.KeyBackspace) && len(a.typing) > 0 {
			rs := []rune(a.typing)
			a.typing = string(rs[:len(rs)-1])
		}
		if a.keys.just(ebiten.KeyEnter) || a.button(nil, "SAVE", (a.W-200)/2, sy+260, 200, 54, btnPrimary) {
			a.TextEntered(a.typing)
			a.go_(a.nameRet)
		}
		if a.keys.just(ebiten.KeyEscape) || a.button(nil, "CANCEL", (a.W-200)/2, sy+326, 200, 46, btnGhost) {
			a.go_(a.nameRet)
		}
		return
	}
	c.text("YOUR NAME", a.W/2, sy+90, 24, true, accent, 1, 1)
	w := min(sw-48, 360)
	c.round((a.W-w)/2, sy+150, w, 64, 18, cardBg, 1)
	cur := ""
	if int(a.t*2)%2 == 0 {
		cur = "|"
	}
	c.text(a.typing+cur, a.W/2, sy+182, 26, true, textCol, 1, 1)
	c.text("letters, digits, space - _ .  (12 max)", a.W/2, sy+234, 13, false, dimText, 1, 1)
	a.button(c, "SAVE", (a.W-200)/2, sy+260, 200, 54, btnPrimary)
	a.button(c, "CANCEL", (a.W-200)/2, sy+326, 200, 46, btnGhost)
}

// ── results ──────────────────────────────────────────────────────────────

type result struct {
	score, best   int
	clears, level int
	secs, rank    int
	newBest       bool
	t             float64
	shown         counter
}

func (a *App) finishDrop(d *dropPlay) {
	s := d.sim
	secs := max(1, int(s.Now))
	prevBest := a.st.best()
	name := a.st.data.Name
	if name == "" {
		name = "?"
	}
	rank := a.st.add(entry{Name: name, Score: s.Score, Clears: s.Clears, Level: s.Level, Secs: secs, At: nowUnix()})
	a.st.save()
	a.res = result{score: s.Score, best: max(prevBest, s.Score), clears: s.Clears, level: s.Level, secs: secs,
		rank: rank, newBest: s.Score > prevBest && s.Score > 0}
	a.res.shown.snap(0)
	a.res.shown.set(a.res.score)
	a.go_(scrOver)
	if a.res.newBest {
		a.sfx(sfxHigh, 1)
		a.haptic(HapticSuccess)
		for i := 0; i < 5; i++ {
			a.fx.burst(a.W*(0.2+0.15*float64(i)), a.H*0.3, 30, fruit[1+i%4], 420, 7, true)
		}
	}
}

func (a *App) overLayout() (x, y, w float64) {
	_, sy, sw, _ := a.safe()
	w = min(sw-32, 400)
	return (a.W - w) / 2, sy + a.H*0.12, w
}

func (a *App) updateOver(dt float64) {
	a.res.t += dt
	a.res.shown.update(dt)
	a.uiOver(nil)
}

func (a *App) drawOver(c *canvas) { a.uiOver(c) }

func (a *App) uiOver(c *canvas) {
	x, y, w := a.overLayout()
	r := &a.res
	if c != nil {
		k := easeOutCubic(r.t / 0.45)
		yy := y + (1-k)*60
		c.card(x, yy, w, 280)
		c.text("GAME OVER", x+w/2, yy+34, 15, true, dimText, float32(k), 1)
		c.text(grouped(r.shown.value()), x+w/2, yy+90, 48, true, textCol, float32(k), 1)
		if r.newBest {
			c.textScaled("NEW BEST!", x+w/2, yy+140, 22, easeOutBack((r.t-0.5)/0.4), accent, float32(clamp01((r.t-0.5)/0.2)))
		} else {
			c.text("BEST "+grouped(r.best), x+w/2, yy+140, 15, false, dimText, float32(k), 1)
		}
		stats := [4][2]string{{"CLEARS", itoa(r.clears)}, {"LEVEL", itoa(r.level)}, {"TIME", itoa(r.secs/60) + ":" + pad2(r.secs%60)}, {"RANK", rankText(r.rank)}}
		for i, s := range stats {
			sx := x + w*(float64(i)+0.5)/4
			c.text(s[0], sx, yy+198, 11.5, true, dimText, float32(k), 1)
			c.text(s[1], sx, yy+224, 19, true, textCol, float32(k), 1)
		}
	}
	if r.t < 0.5 && c == nil {
		return
	}
	if a.button(c, "PLAY AGAIN", x+24, y+300, w-48, 58, btnPrimary) {
		a.startDrop()
	}
	if a.button(c, "MENU", x+24, y+370, w-48, 50, btnSecondary) {
		a.go_(scrMenu)
	}
}

func pad2(n int) string {
	if n < 10 {
		return "0" + itoa(n)
	}
	return itoa(n)
}

func rankText(r int) string {
	if r <= 0 {
		return "–"
	}
	return "#" + itoa(r)
}
