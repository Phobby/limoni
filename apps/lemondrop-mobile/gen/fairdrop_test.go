package gen

import (
	"testing"

	"github.com/thebanri/limoni/apps/lemondrop-mobile/core"
)

// bot plays a run with a simple, fixed strategy (lowest landing, matching the
// colour beneath when it can) and returns the pieces placed. It is the same
// player for every generator, so differences come from the generator.
func bot(seed uint64, g core.Generator, maxPieces int) (pieces, score int) {
	s := core.NewSim(seed, 4)
	s.Gen = g
	s.Start()
	for s.Phase == core.Playing && s.Pieces < maxPieces {
		bestX, bestR, bestV := s.Cur.X, 0, -1<<30
		for rot := 0; rot < 4; rot++ {
			for x := -3 * s.B; x < s.GW; x += s.Step() {
				if !s.Fits(s.Cur.Kind, rot, x, s.Cur.Y) {
					continue
				}
				y := s.Cur.Y
				for s.Fits(s.Cur.Kind, rot, x, y+1) {
					y++
				}
				v := y * 4
				for _, c := range core.Shapes[s.Cur.Kind][rot] {
					gx, gy := x+int(c[0])*s.B+s.B/2, y+(int(c[1])+1)*s.B
					if gy < s.GH && s.Grain(gx, gy)&core.ColourMask == s.Cur.Colour {
						v += 3 * s.B
					}
				}
				if v > bestV {
					bestX, bestR, bestV = x, rot, v
				}
			}
		}
		s.Cur.Rot, s.Cur.X = bestR, bestX
		s.HardDrop()
		for i := 0; i < 400 && !s.Settled; i++ {
			s.Tick()
		}
	}
	return s.Pieces, s.Score
}

func TestFairDropIsSeeded(t *testing.T) {
	seq := func() []core.Piece {
		s := core.NewSim(11, core.MaxB)
		s.Gen = &FairDrop{P: Relaxed}
		s.Start()
		var out []core.Piece
		for i := 0; i < 60 && s.Phase == core.Playing; i++ {
			out = append(out, s.Next)
			s.HardDrop()
			for j := 0; j < 30; j++ {
				s.Tick()
			}
		}
		return out
	}
	a, b := seq(), seq()
	if len(a) != len(b) {
		t.Fatalf("lengths %d %d", len(a), len(b))
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("piece %d differs: %+v vs %+v", i, a[i], b[i])
		}
	}
}

// FairDrop never changes how many of each shape the player gets: every
// aligned window of seven pieces holds each shape once, as the classic bag.
func TestFairDropKeepsBagFairnessAndColourBalance(t *testing.T) {
	s := core.NewSim(5, 4)
	f := &FairDrop{P: Relaxed}
	s.Gen = f
	s.Start() // Start deals two pieces (next, then current's replacement)
	var colours [core.Colours + 1]int
	r := core.RNG(77)
	// Deal straight from the generator on a moderately full board.
	for y := s.GH / 2; y < s.GH; y++ {
		for x := 0; x < s.GW; x += 2 {
			s.Sand[y*s.GW+x] = uint8(1 + (x/9)%core.Colours)
		}
	}
	for len(s.Bag(&r)) != 7 { // align windows to a bag boundary (Start dealt two)
		f.Next(s, &r)
	}
	for w := 0; w < 300; w++ {
		var seen [7]int
		for i := 0; i < 7; i++ {
			p := f.Next(s, &r)
			seen[p.Kind]++
			colours[p.Colour]++
		}
		for k, n := range seen {
			if n != 1 {
				t.Fatalf("window %d: shape %d dealt %d times", w, k, n)
			}
		}
	}
	total := 300 * 7
	for c := 1; c <= core.Colours; c++ {
		share := float64(colours[c]) / float64(total)
		if share < 0.12 || share > 0.40 {
			t.Errorf("colour %d is %.0f%% of pieces", c, 100*share)
		}
	}
}

// The assisted profiles measurably help the same player survive longer.
func TestFairDropHelpsTheSamePlayer(t *testing.T) {
	if testing.Short() {
		t.Skip("plays many runs")
	}
	const runs, cap = 12, 400
	var classic, relaxed int
	for i := uint64(1); i <= runs; i++ {
		p, _ := bot(i, nil, cap)
		classic += p
		p, _ = bot(i, &FairDrop{P: Relaxed}, cap)
		relaxed += p
	}
	t.Logf("pieces survived over %d runs: classic %d, relaxed %d", runs, classic, relaxed)
	if relaxed < classic {
		t.Fatalf("Relaxed (%d) did not help over classic (%d)", relaxed, classic)
	}
}

func BenchmarkFairDropNext(b *testing.B) {
	s := core.NewSim(1, core.MaxB)
	f := &FairDrop{P: Relaxed}
	s.Gen = f
	s.Start()
	for y := s.GH / 2; y < s.GH; y++ {
		for x := 0; x < s.GW; x++ {
			s.Sand[y*s.GW+x] = uint8(1 + (x/7+y/9)%core.Colours)
		}
	}
	r := core.RNG(3)
	b.ReportAllocs()
	for b.Loop() {
		f.P.Calm = 1 // always evaluate
		f.Next(s, &r)
	}
}
