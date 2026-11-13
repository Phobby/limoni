package app

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/thebanri/limoni/apps/lemondrop-mobile/core"
)

// ── persistence ──────────────────────────────────────────────────────────

func TestSaveRoundTripAndVersion(t *testing.T) {
	dir := t.TempDir()
	s := newStore(dir)
	s.data.Name = "Ayşe"
	s.data.Settings.Buttons = true
	s.add(ModeZest, entry{Name: "Ayşe", Score: 900})
	s.save()

	r := newStore(dir)
	if r.data.Name != "Ayşe" || !r.data.Settings.Buttons || r.best(ModeZest) != 900 {
		t.Fatalf("reloaded %+v", r.data)
	}
	b, _ := os.ReadFile(filepath.Join(dir, "lemondrop-mobile.json"))
	var raw map[string]any
	if json.Unmarshal(b, &raw) != nil || raw["version"].(float64) != saveVersion {
		t.Fatalf("save file is not versioned: %s", b)
	}
	// A file from a future version is not trusted.
	os.WriteFile(filepath.Join(dir, "lemondrop-mobile.json"), []byte(`{"version":99,"name":"x"}`), 0o644)
	if f := newStore(dir); f.data.Name == "x" {
		t.Fatal("loaded a save from an unknown future version")
	}
}

func TestLegacyTerminalSaveIsImportedOnce(t *testing.T) {
	dir := t.TempDir()
	legacy := `{"name":"  Lemon!Fan ","board":[{"name":"Lemon!Fan","score":2312,"clears":6,"level":2,"secs":90}]}`
	os.WriteFile(filepath.Join(dir, "lemondrop.json"), []byte(legacy), 0o644)
	s := newStore(dir)
	if s.data.Name != "LemonFan" || s.best(ModeClassic) != 2312 {
		t.Fatalf("legacy import: name %q best %d", s.data.Name, s.best(ModeClassic))
	}
	s2 := newStore(dir) // second start: not imported again
	if n := len(s2.data.Boards[ModeClassic]); n != 1 {
		t.Fatalf("classic board has %d entries after a second start", n)
	}
}

func TestBoardsRankAndKeepTwenty(t *testing.T) {
	s := newStore("")
	for i := 0; i < 30; i++ {
		s.add(ModeFair, entry{Name: "p", Score: i * 10})
	}
	b := s.data.Boards[ModeFair]
	if len(b) != boardSize || b[0].Score != 290 || b[boardSize-1].Score != 100 {
		t.Fatalf("board %d long, top %d, last %d", len(b), b[0].Score, b[len(b)-1].Score)
	}
	if r := s.add(ModeFair, entry{Name: "low", Score: 5}); r != 0 {
		t.Fatalf("a score below the board ranked %d", r)
	}
	if r := s.add(ModeFair, entry{Name: "top", Score: 1000}); r != 1 {
		t.Fatalf("a new best ranked %d", r)
	}
}

// ── touch gestures ───────────────────────────────────────────────────────

// testDrop is a falling-mode screen on a phone-sized layout without a GPU.
func testDrop(t *testing.T) (*App, *dropPlay) {
	t.Helper()
	a := &App{plat: &Desktop{Insets: [4]float64{59, 0, 34, 0}}, W: 393, H: 852, scale: 3, fx: &fxSystem{}}
	a.st = newStore("")
	a.st.data.Settings.Haptics = false
	a.st.data.Settings.Muted = true
	d := &dropPlay{a: a, mode: ModeClassic}
	d.sim = core.NewSim(1, core.MaxB)
	d.sim.Start()
	d.layout()
	return a, d
}

// finger simulates one pointer over frames: down at (x0,y0), then moves.
func finger(a *App, x, y float64, phase string, vy float64) {
	p := &a.in.ptrs[0]
	switch phase {
	case "down":
		*p = pointer{id: 1, x: x, y: y, sx: x, sy: y, px: x, py: y, t0: a.t, t: a.t, down: true, active: true}
	case "move":
		p.down = false
		p.px, p.py = p.x, p.y
		p.x, p.y, p.t, p.vy = x, y, a.t, vy
	case "up":
		p.down, p.up = false, true
		p.t = a.t
	}
}

func TestDragSteersThePiece(t *testing.T) {
	a, d := testDrop(t)
	s := d.sim
	x0 := s.Cur.X
	cx, cy := d.bx+d.bw/2, d.by+d.bh/2
	finger(a, cx, cy, "down", 0)
	d.gestures()
	// three blocks to the right, in dp: 3 × B grains × g dp a grain
	dx := 3 * float64(s.B) * d.g
	for i := 1; i <= 6; i++ {
		a.t += 1.0 / 60
		finger(a, cx+dx*float64(i)/6, cy, "move", 0)
		d.gestures()
	}
	if got := s.Cur.X - x0; got != 3*s.B && s.Fits(s.Cur.Kind, s.Cur.Rot, x0+3*s.B, s.Cur.Y) {
		t.Fatalf("dragging three blocks moved the piece %d grains, want %d", got, 3*s.B)
	}
}

func TestFlickDownHardDropsOnce(t *testing.T) {
	a, d := testDrop(t)
	s := d.sim
	cx, cy := d.bx+d.bw/2, d.by+d.bh/3
	finger(a, cx, cy, "down", 0)
	d.gestures()
	a.t += 1.0 / 60
	finger(a, cx, cy+80, "move", 2400)
	d.gestures()
	if s.Pieces != 1 {
		t.Fatalf("a flick placed %d pieces, want 1", s.Pieces)
	}
	a.t += 1.0 / 60
	finger(a, cx, cy+160, "move", 2400)
	d.gestures()
	if s.Pieces != 1 {
		t.Fatal("one flick dropped two pieces")
	}
}

func TestTapTurnsAndSlowDragSoftDrops(t *testing.T) {
	a, d := testDrop(t)
	s := d.sim
	cx, cy := d.bx+d.bw*0.75, d.by+d.bh/2
	finger(a, cx, cy, "down", 0)
	d.gestures()
	a.t += 0.08
	finger(a, cx, cy, "up", 0)
	rot := s.Cur.Rot
	d.gestures()
	if s.Cur.Rot == rot && s.Cur.Kind != 3 { // the O looks the same but still turns
		t.Fatal("a tap on the right half did not turn the piece")
	}
	finger(a, cx, cy, "down", 0)
	d.gestures()
	a.t += 0.2
	finger(a, cx, cy+60, "move", 300)
	if !d.gestures() {
		t.Fatal("a slow drag down did not soft-drop")
	}
}

// ── layout ───────────────────────────────────────────────────────────────

func TestBoardFitsTheSafeAreaOnManyPhones(t *testing.T) {
	phones := [][4]float64{ // w, h, top inset, bottom inset (dp)
		{375, 667, 20, 0},   // iPhone SE
		{393, 852, 59, 34},  // iPhone 15
		{430, 932, 59, 34},  // iPhone 15 Pro Max
		{360, 800, 24, 16},  // common Android
		{412, 915, 28, 20},  // Pixel
		{820, 1180, 24, 20}, // iPad portrait
	}
	for _, ph := range phones {
		a, d := testDrop(t)
		a.W, a.H = ph[0], ph[1]
		a.plat = &Desktop{Insets: [4]float64{ph[2], 0, ph[3], 0}}
		d.layout()
		if d.bx < 0 || d.bx+d.bw > a.W || d.by < ph[2] || d.by+d.bh > a.H-ph[3] {
			t.Errorf("%v: board %.0f,%.0f %.0fx%.0f leaves the safe area", ph, d.bx, d.by, d.bw, d.bh)
		}
		if r := d.bh / d.bw; r < 1.79 || r > 1.81 {
			t.Errorf("%v: board aspect %.3f, want 1.8", ph, r)
		}
		if d.g*float64(d.sim.GW) != d.bw {
			t.Errorf("%v: grain size does not tile the board", ph)
		}
	}
}

// ── leaderboard policy ───────────────────────────────────────────────────

func TestOnlyClassicRunsGoToTheWorldBoard(t *testing.T) {
	posts := make(chan string, 4)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			b, _ := io.ReadAll(r.Body)
			posts <- string(b)
		}
		w.Write([]byte(`{"rank":3,"board":[]}`))
	}))
	defer srv.Close()

	for _, m := range []Mode{ModeFair, ModeClassic} {
		a, d := testDrop(t)
		a.remote = newRemote(srv.URL)
		a.st.data.Name = "tester"
		d.mode = m
		for d.sim.Phase == core.Playing {
			d.sim.HardDrop()
		}
		a.finishDrop(d)
	}
	select {
	case body := <-posts:
		var run remoteRun
		if json.Unmarshal([]byte(body), &run) != nil || run.Name != "tester" || run.Pieces == 0 {
			t.Fatalf("bad submission %s", body)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the classic run was not submitted")
	}
	select {
	case body := <-posts:
		t.Fatalf("a second (non-classic) run was submitted: %s", body)
	case <-time.After(300 * time.Millisecond):
	}
}
