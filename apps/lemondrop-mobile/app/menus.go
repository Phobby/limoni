package app

import (
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/thebanri/limoni/apps/lemondrop-mobile/core"
	"github.com/thebanri/limoni/apps/lemondrop-mobile/gen"
)

// Menus are immediate-mode: one function per screen lays out, hit-tests
// (c == nil, from Update) and draws (from Draw), so taps and pixels agree.

func (a *App) updateMenus() {
	switch a.scr {
	case scrMenu:
		a.uiMenu(nil)
	case scrModes:
		a.uiModes(nil)
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
	// the attract board, dim, behind every menu
	if a.scr == scrMenu || a.scr == scrModes {
		_, sy, _, sh := a.safe()
		// full width at the bottom, faint: a bed of sand under the menu
		g := a.W / float64(a.attract.GW)
		h := g * float64(a.attract.GH)
		a.atex.fill(a.attract, a.t)
		a.atex.draw(c, 0, sy+sh-h+g*float64(a.attract.GH)*0.25, g, a.t, 0.38)
	}
	switch a.scr {
	case scrMenu:
		a.uiMenu(c)
	case scrModes:
		a.uiModes(c)
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
		if a.settingsRet == scrDrop || a.settingsRet == scrZest {
			a.go_(a.settingsRet)
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
	} else if a.in.tapped(a.W/2-90, y-58, 180, 34) {
		a.askName(scrMenu)
	}
	if a.button(c, "PLAY", x, y, w, 62, btnPrimary) {
		a.go_(scrModes)
	}
	if a.button(c, "LEADERBOARD", x, y+76, w, 52, btnSecondary) {
		a.go_(scrBoard)
		if a.remote != nil && a.worldState != worldLoading {
			a.worldState = worldLoading
			a.remote.fetch()
		}
	}
	if a.button(c, "HOW TO PLAY", x, y+140, w, 52, btnSecondary) {
		a.go_(scrHelp)
	}
	if a.button(c, "SETTINGS", x, y+204, w, 52, btnSecondary) {
		a.settingsRet = scrMenu
		a.go_(scrSettings)
	}
}

// ── modes ────────────────────────────────────────────────────────────────

func (a *App) uiModes(c *canvas) {
	if a.header(c, "CHOOSE A MODE") {
		a.back()
		return
	}
	_, sy, sw, _ := a.safe()
	w := min(sw-32, 420)
	x := (a.W - w) / 2
	y := sy + 84
	type card struct {
		title, line1, line2 string
		col                 rgb
		mode                Mode
	}
	cards := [3]card{
		{"CLASSIC DROP", "The original Lemon Drop rules.", "World leaderboard.", fruit[1], ModeClassic},
		{"FAIR DROP", "Same rules; pieces dealt with care.", "Help: " + a.st.data.Settings.FairProfile + " · local board.", fruit[2], ModeFair},
		{"ZEST BLOCKS", "Drag three pieces, clear lines.", "Dealing: " + a.st.data.Settings.ZestProfile + " · local board.", fruit[3], ModeZest},
	}
	for i, cd := range cards {
		cy := y + float64(i)*132
		if c == nil {
			if a.in.tapped(x, cy, w, 116) {
				a.sfx(sfxTap, 0.6)
				a.haptic(HapticLight)
				if cd.mode == ModeZest {
					a.startZest()
				} else {
					a.startDrop(cd.mode)
				}
				return
			}
			continue
		}
		press := a.in.pressing(x, cy, w, 116) != nil
		k := 0.0
		if press {
			k = 3
		}
		c.card(x+k, cy+k, w-2*k, 116-2*k)
		c.round(x+k+14, cy+k+18, 8, 80, 4, cd.col, 1)
		c.text(cd.title, x+40, cy+32, 21, true, cd.col.lighten(0.2), 1, 0)
		c.text(cd.line1, x+40, cy+62, 14.5, false, textCol, 1, 0)
		c.text(cd.line2, x+40, cy+86, 13, false, dimText, 1, 0)
		c.text("BEST "+grouped(a.st.best(cd.mode)), x+w-18, cy+32, 12.5, true, dimText, 1, 2)
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
	const content = 672.0
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
	if v := a.segmented(c, "Falling-mode controls", x, y+252, w, []string{"Gestures", "Buttons"}, ctl); c == nil && v != ctl {
		s.Buttons, changed = v == 1, true
	}
	hand := 0
	if s.LeftHanded {
		hand = 1
	}
	if v := a.segmented(c, "Button layout", x, y+330, w, []string{"Right hand", "Left hand"}, hand); c == nil && v != hand {
		s.LeftHanded, changed = v == 1, true
	}
	fair := []string{gen.Relaxed.Name, gen.Balanced.Name}
	fi := index(fair, s.FairProfile)
	if v := a.segmented(c, "Fair Drop help", x, y+408, w, fair, fi); c == nil && v != fi {
		s.FairProfile, changed = fair[v], true
	}
	zest := []string{gen.ZestRelaxed.Name, gen.ZestStandard.Name, gen.ZestChallenge.Name}
	zi := index(zest, s.ZestProfile)
	if v := a.segmented(c, "Zest Blocks dealing", x, y+486, w, zest, zi); c == nil && v != zi {
		s.ZestProfile, changed = zest[v], true
	}
	setB(&s.Online, a.toggle(c, "World leaderboard (Classic)", x, y+564, w, s.Online))
	name := a.st.data.Name
	if name == "" {
		name = "not set"
	}
	if c != nil {
		c.text("Player name", x+16, y+636, 17, false, textCol, 1, 0)
		c.text(name+"  ›", x+w-16, y+636, 16, true, accent, 1, 2)
	} else if a.in.tapped(x, y+612, w, 50) {
		a.askName(scrSettings)
	}
	if changed {
		a.fx.reduce = s.ReduceMove
		a.applyVolumes()
		if s.Online && a.remote == nil {
			url := a.opt.BoardURL
			if url == "" {
				url = defaultBoard
			}
			a.remote = newRemote(url)
		} else if !s.Online {
			a.remote = nil
		}
		a.st.save()
	}
}

func index(opts []string, v string) int {
	for i, o := range opts {
		if o == v {
			return i
		}
	}
	return 0
}

// ── how to play ──────────────────────────────────────────────────────────

var helpPages = [3]struct{ title, body string }{
	{"FALLING SAND", `Pieces fall like in any falling-block game, but when one lands it
crumbles into sand of its colour and runs down the pile.

A run of ONE colour that reaches from the left wall to the right wall
flashes and clears. Grains touching at a corner still connect.

Whatever rested on it falls — and may make another clear: a CHAIN.

Gestures: drag sideways to steer · tap the left/right half to turn ·
flick down to drop · drag down and hold to fall faster.

Score: base × level × chain. Base is the cleared grains, ten points a
block's worth. Level rises every four clears. Dropping a piece scores
2 a row. Classic uses the terminal game's exact rules.`},
	{"FAIR DROP", `Same rules as Classic. The difference is how pieces are dealt.

Every seven pieces still contain each shape exactly once. When the
stack gets high, Fair Drop picks which of the remaining shapes comes
next, and its colour, by looking at where it could land:
widening a band of its colour, keeping the stack low.

Relaxed helps often; Balanced helps less. Nothing is ever dealt to
make you lose. Fair Drop scores go on their own local board.`},
	{"ZEST BLOCKS", `Drag the three pieces onto the 8×8 board. Full rows and columns
clear — both at once when a piece completes both.

Placing: 1 point a cell.
Clearing L lines at once: 80 × L × (L + Z) × combo,
where Z counts ZEST LINES — lines all of one colour.
Emptying the board: +300.

COMBO grows with every placement that clears. Three placements in a
row without a clear end it.

The run ends when no piece left in the tray fits anywhere.
Relaxed and Standard dealing only give trays that can be placed
completely; Challenge does not promise that.`},
}

func (a *App) uiHelp(c *canvas) {
	if a.header(c, "HOW TO PLAY") {
		a.back()
		return
	}
	_, sy, sw, sh := a.safe()
	w := min(sw-24, 520)
	x := (a.W - w) / 2
	tabs := []string{"Falling", "Fair Drop", "Zest"}
	if v := a.segmented(c, "", x, sy+64, w, tabs, a.helpTab); c == nil {
		a.helpTab = v
	}
	if c == nil {
		return
	}
	y := sy + 160
	c.card(x, y, w, min(sh-180, 560))
	p := helpPages[a.helpTab]
	c.text(p.title, x+20, y+28, 18, true, accent, 1, 0)
	const size = 14.5
	ly := y + 62
	for _, para := range strings.Split(strings.ReplaceAll(p.body, "\n\n", "\x00"), "\x00") {
		for _, line := range wrap(strings.ReplaceAll(para, "\n", " "), w-40, size, c.s) {
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

// ── leaderboard ──────────────────────────────────────────────────────────

func (a *App) uiBoard(c *canvas) {
	if a.header(c, "LEADERBOARD") {
		a.back()
		return
	}
	_, sy, sw, sh := a.safe()
	w := min(sw-24, 520)
	x := (a.W - w) / 2
	modes := []Mode{ModeClassic, ModeFair, ModeZest}
	cur := 0
	for i, m := range modes {
		if m == a.boardTab {
			cur = i
		}
	}
	if v := a.segmented(c, "", x, sy+64, w, []string{"Classic", "Fair Drop", "Zest"}, cur); c == nil {
		a.boardTab = modes[v]
		return
	}
	y := sy + 160
	h := min(sh-180, 640)
	c.card(x, y, w, h)
	rows, status := a.st.data.Boards[a.boardTab], "on this device"
	if a.boardTab == ModeClassic {
		switch a.worldState {
		case worldOK:
			rows, status = a.world, "world"
		case worldLoading:
			status = "connecting… showing this device"
		case worldOffline:
			status = "offline — showing this device"
		}
	}
	c.text(strings.ToUpper(status), x+w/2, y+24, 11.5, true, dimText, 1, 1)
	if len(rows) == 0 {
		c.text("No runs yet — be the first.", x+w/2, y+h/2, 16, false, dimText, 1, 1)
		return
	}
	rh := min(28.0, (h-60)/float64(boardSize))
	me := a.st.data.Name
	for i, e := range rows {
		if i >= boardSize {
			break
		}
		ry := y + 52 + float64(i)*rh
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
	if a.plat.RequestText("Your name for the leaderboard", a.st.data.Name) {
		return
	}
	a.typing, a.nameRet = a.st.data.Name, ret
	a.go_(scrName)
}

var chars []rune

func (a *App) uiName(c *canvas) {
	sx, sy, sw, _ := a.safe()
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
	_ = sx
}

// ── results ──────────────────────────────────────────────────────────────

type result struct {
	mode          Mode
	score, best   int
	clears, level int
	secs, rank    int
	newBest       bool
	submitted     bool
	t             float64
	shown         counter
}

func (a *App) finishDrop(d *dropPlay) {
	s := d.sim
	secs := max(1, int(s.Now))
	prevBest := a.st.best(d.mode)
	name := a.st.data.Name
	if name == "" {
		name = "?"
	}
	rank := a.st.add(d.mode, entry{Name: name, Score: s.Score, Clears: s.Clears, Level: s.Level, Secs: secs, At: nowUnix()})
	a.st.save()
	a.res = result{mode: d.mode, score: s.Score, best: max(prevBest, s.Score), clears: s.Clears, level: s.Level, secs: secs, rank: rank, newBest: s.Score > prevBest && s.Score > 0}
	// Only Classic shares the server's rules (see docs/ARCHITECTURE.md §3.8).
	if d.mode == ModeClassic && a.remote != nil && a.st.data.Name != "" && s.NLog <= core.MaxLog && s.Pieces > 0 {
		a.worldRank = -1
		a.remote.submit(remoteRun{Name: a.st.data.Name, Secs: secs, Pieces: s.Pieces, Drops: s.DropPts,
			Clears: append([][2]int32{}, s.ClearLog[:s.NLog]...)})
		a.res.submitted = true
	}
	a.enterOver()
}

func (a *App) finishZest(z *zestPlay) {
	g := z.g
	secs := max(1, int(a.t-z.start))
	prevBest := a.st.best(ModeZest)
	name := a.st.data.Name
	if name == "" {
		name = "?"
	}
	rank := a.st.add(ModeZest, entry{Name: name, Score: g.Score, Clears: g.Lines, Level: g.Placed, Secs: secs, At: nowUnix()})
	a.st.save()
	a.res = result{mode: ModeZest, score: g.Score, best: max(prevBest, g.Score), clears: g.Lines, level: g.Placed, secs: secs, rank: rank, newBest: g.Score > prevBest && g.Score > 0}
	a.enterOver()
}

func (a *App) enterOver() {
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
		c.card(x, yy, w, 430)
		title := map[Mode]string{ModeClassic: "CLASSIC DROP", ModeFair: "FAIR DROP", ModeZest: "ZEST BLOCKS"}[r.mode]
		c.text(title, x+w/2, yy+34, 15, true, dimText, float32(k), 1)
		c.text(grouped(r.shown.value()), x+w/2, yy+90, 48, true, textCol, float32(k), 1)
		if r.newBest {
			c.textScaled("NEW BEST!", x+w/2, yy+140, 22, easeOutBack((r.t-0.5)/0.4), accent, float32(clamp01((r.t-0.5)/0.2)))
		} else {
			c.text("BEST "+grouped(r.best), x+w/2, yy+140, 15, false, dimText, float32(k), 1)
		}
		l1, l2 := "CLEARS", "LEVEL"
		if r.mode == ModeZest {
			l1, l2 = "LINES", "PIECES"
		}
		stats := [4][2]string{{l1, itoa(r.clears)}, {l2, itoa(r.level)}, {"TIME", itoa(r.secs/60) + ":" + pad2(r.secs%60)}, {"RANK", rankText(r.rank)}}
		for i, s := range stats {
			sx := x + w*(float64(i)+0.5)/4
			c.text(s[0], sx, yy+188, 11.5, true, dimText, float32(k), 1)
			c.text(s[1], sx, yy+214, 19, true, textCol, float32(k), 1)
		}
		if r.mode == ModeClassic {
			msg := "World board: set a name to take part"
			switch {
			case !r.submitted && a.remote == nil:
				msg = "World board: off"
			case r.submitted && a.worldRank == -1:
				msg = "World board: sending…"
			case r.submitted && a.worldRank > 0:
				msg = "World rank #" + itoa(a.worldRank)
			case r.submitted && a.worldState == worldOffline:
				msg = "World board: offline — kept on this device"
			case r.submitted:
				msg = "World board: not in the top " + itoa(boardSize)
			}
			c.text(msg, x+w/2, yy+252, 13, false, dimText, float32(k), 1)
		} else {
			c.text("Local board — rules differ from Classic", x+w/2, yy+252, 13, false, dimText, float32(k), 1)
		}
	}
	if r.t < 0.5 && c == nil {
		return
	}
	if a.button(c, "PLAY AGAIN", x+24, y+290, w-48, 58, btnPrimary) {
		if r.mode == ModeZest {
			a.startZest()
		} else {
			a.startDrop(r.mode)
		}
	}
	if a.button(c, "MENU", x+24, y+360, w-48, 50, btnSecondary) {
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
