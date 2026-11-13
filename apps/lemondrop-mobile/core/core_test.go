package core

import (
	"hash/fnv"
	"testing"
)

// playing is a run on a 4-grain board, as the terminal game's tests use.
func playing() *Sim {
	s := NewSim(42, 4)
	s.Start()
	return s
}

func grains(s *Sim) (n int) {
	for _, v := range s.Sand[:s.GW*s.GH] {
		if v != 0 {
			n++
		}
	}
	return n
}

func settle(t *testing.T, s *Sim) {
	t.Helper()
	s.WakeAll()
	for i := 0; i < 2000; i++ {
		s.SandTick()
		if s.Settled {
			return
		}
	}
	t.Fatal("the sand never settled")
}

func stripe(s *Sim, y, x0, x1 int, v uint8) {
	for x := x0; x < x1; x++ {
		s.Sand[y*s.GW+x] = v
	}
}

// TestParityWithTerminalGame replays a scripted run and compares it with a
// trace recorded from apps/lemondrop at the same seed and inputs (recorded by
// a throw-away test in that package; see docs/ARCHITECTURE.md §3.1).
func TestParityWithTerminalGame(t *testing.T) {
	s := playing()
	for i := 1; i <= 6000 && s.Phase == Playing; i++ {
		switch {
		case i%90 == 0:
			s.HardDrop()
		case i%37 == 0:
			s.Shift(s.Step())
		case i%53 == 0:
			s.Turn(1)
		case i%71 == 0:
			s.Shift(-2 * s.Step())
		}
		s.SoftDrop = i%300 < 40
		s.Tick()
	}
	h := fnv.New64a()
	h.Write(s.Sand[:s.GW*s.GH])
	got := struct {
		score, clears, level, pieces, drops, nlog int
		ticks                                     uint64
		hash                                      uint64
	}{s.Score, s.Clears, s.Level, s.Pieces, s.DropPts, s.NLog, s.Ticks, h.Sum64()}
	want := struct {
		score, clears, level, pieces, drops, nlog int
		ticks                                     uint64
		hash                                      uint64
	}{11397, 20, 6, 88, 586, 20, 6000, 0x40d3be5374e0133f}
	if got != want {
		t.Fatalf("diverged from the terminal game:\n got %+v\nwant %+v", got, want)
	}
}

func TestSandFallsAndIsConserved(t *testing.T) {
	s := playing()
	s.Phase = Attract
	for y := 5; y < 25; y++ {
		s.Sand[y*s.GW+20] = uint8(1 + y%3)
		s.Sand[y*s.GW+21] = uint8(1 + (y+1)%3)
	}
	before := grains(s)
	settle(t, s)
	if n := grains(s); n != before {
		t.Fatalf("%d grains before, %d after", before, n)
	}
	for y := 0; y < s.GH-1; y++ {
		for x := 0; x < s.GW; x++ {
			if s.Grain(x, y) != 0 && s.Grain(x, y+1) == 0 {
				t.Fatalf("grain at (%d,%d) rests on nothing", x, y)
			}
		}
	}
}

func TestAWallToWallRunClearsAndScores(t *testing.T) {
	s := playing()
	y := s.GH - 1
	stripe(s, y, 0, 20, 2)
	stripe(s, y-1, 20, s.GW, 2) // joins only at a corner
	stripe(s, y, 20, s.GW, 3)
	s.WakeAll()
	s.findClear()
	if s.FlashT == 0 || s.Clears != 1 || s.Score == 0 {
		t.Fatalf("flash %v clears %d score %d", s.FlashT, s.Clears, s.Score)
	}
	for i := 0; i < int(FlashTime*Hz)+2; i++ {
		s.SandTick()
	}
	for x := 0; x < s.GW; x++ {
		if s.Grain(x, y)&ColourMask == 2 {
			t.Fatalf("run grain left at (%d,%d)", x, y)
		}
	}
}

func TestARunThatStopsShortDoesNotClear(t *testing.T) {
	s := playing()
	y := s.GH - 1
	stripe(s, y, 0, s.GW-1, 1)
	stripe(s, y, s.GW-1, s.GW, 2)
	s.findClear()
	if s.FlashT != 0 || s.Score != 0 {
		t.Fatal("a run short of the right wall cleared")
	}
}

func TestAChainCountsAndResetsOnLanding(t *testing.T) {
	s := playing()
	y := s.GH - 1
	stripe(s, y, 0, s.GW, 1)
	stripe(s, y-1, 0, 20, 2)
	stripe(s, y-1, 21, s.GW, 2)
	s.Sand[20] = 2
	s.WakeAll()
	for i := 0; i < 600 && s.Clears < 2; i++ {
		s.SandTick()
	}
	if s.Clears != 2 || s.Combo != 2 {
		t.Fatalf("clears %d combo %d, want a chain of 2", s.Clears, s.Combo)
	}
	if s.ClearLog[1][1] != 2 {
		t.Fatalf("second clear logged with chain %d", s.ClearLog[1][1])
	}
	s.HardDrop()
	if s.Combo != 0 {
		t.Fatalf("combo %d after a landing, want 0", s.Combo)
	}
}

// The shared scoreboard recomputes the score from the log; it must agree.
func TestServerFormulaMatchesScore(t *testing.T) {
	s := NewSim(7, MaxB)
	s.Start()
	for i := 1; i < 20000 && s.Phase == Playing; i++ {
		if i%23 == 0 {
			s.Shift(s.Step() * (i%5 - 2))
		}
		if i%61 == 0 {
			s.HardDrop()
		}
		s.Tick()
	}
	if s.Clears == 0 {
		t.Skip("no clear in this run")
	}
	score := s.DropPts
	for i := 0; i < s.NLog; i++ {
		c := s.ClearLog[i]
		score += int(c[0]) * (1 + (i+1)/4) * int(c[1])
	}
	if score != s.Score {
		t.Fatalf("server formula %d, game %d", score, s.Score)
	}
}

func TestSameSeedSameRun(t *testing.T) {
	run := func() (int, uint64) {
		s := NewSim(99, MaxB)
		s.Start()
		for i := 1; i < 5000 && s.Phase == Playing; i++ {
			if i%45 == 0 {
				s.HardDrop()
			}
			s.Tick()
		}
		h := fnv.New64a()
		h.Write(s.Sand[:s.GW*s.GH])
		return s.Score, h.Sum64()
	}
	a1, h1 := run()
	a2, h2 := run()
	if a1 != a2 || h1 != h2 {
		t.Fatalf("same seed gave %d/%x and %d/%x", a1, h1, a2, h2)
	}
}

// Frame-rate independence: the same total time in different slices gives the
// same state, because Update runs whole fixed steps.
func TestUpdateIsFrameRateIndependent(t *testing.T) {
	a, b := NewSim(5, MaxB), NewSim(5, MaxB)
	a.Start()
	b.Start()
	for i := 0; i < 600; i++ { // 10 s at 60 fps
		a.Update(1.0 / 60)
	}
	for i := 0; i < 1200; i++ { // 10 s at 120 fps
		b.Update(1.0 / 120)
	}
	if a.Ticks != b.Ticks || a.Score != b.Score || a.Cur != b.Cur || a.Sand != b.Sand {
		t.Fatalf("60 fps: ticks %d score %d; 120 fps: ticks %d score %d", a.Ticks, a.Score, b.Ticks, b.Score)
	}
}

func TestAFullBoardEndsTheRun(t *testing.T) {
	s := playing()
	for i := 0; i < 400 && s.Phase == Playing; i++ {
		s.HardDrop()
	}
	if s.Phase != Over {
		t.Fatal("dropping pieces forever never ended the run")
	}
	over := false
	for e, ok := s.PollEvent(); ok; e, ok = s.PollEvent() {
		over = over || e.Kind == EvGameOver
	}
	if !over {
		t.Fatal("no game-over event")
	}
}

func TestTurningOffAWallKicks(t *testing.T) {
	s := playing()
	s.Cur = Piece{Kind: 0, Rot: 1, Colour: 1, X: -s.B, Y: 10}
	for !s.Fits(s.Cur.Kind, s.Cur.Rot, s.Cur.X, s.Cur.Y) {
		s.Cur.X++
	}
	if !s.Turn(1) {
		t.Fatal("an upright I against the wall could not turn")
	}
}

func TestStepsAllocateNothing(t *testing.T) {
	s := NewSim(3, MaxB)
	s.Start()
	n := testing.AllocsPerRun(500, func() {
		s.Tick()
		if s.Phase != Playing {
			s.Start()
		}
		for _, ok := s.PollEvent(); ok; _, ok = s.PollEvent() {
		}
	})
	if n != 0 {
		t.Fatalf("a step allocates %.1f times", n)
	}
}

func BenchmarkTickBusyBoard(b *testing.B) {
	s := NewSim(1, MaxB)
	s.Start()
	for y := s.GH / 3; y < s.GH; y++ { // a busy two-thirds-full board
		for x := 0; x < s.GW; x++ {
			s.Sand[y*s.GW+x] = uint8(1 + (x/7+y/5)%Colours)
		}
	}
	s.WakeAll()
	b.ReportAllocs()
	for b.Loop() {
		s.WakeAll()
		s.Tick()
	}
}
