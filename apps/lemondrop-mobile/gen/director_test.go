package gen

import (
	"testing"

	"github.com/thebanri/limoni/apps/lemondrop-mobile/core"
)

// bot plays a run with a fixed, simple strategy (low landing, matching the
// colour beneath), the same for every generator, and reports what happened.
type run struct {
	pieces, clears, chains, score int
	colours                       []uint8
	clearAt                       []int // piece index of each landing that led to a clear
	modes                         [4]int
	over                          bool
}

func bot(seed uint64, g core.Generator, maxPieces int) run {
	s := core.NewSim(seed, 4)
	s.Gen = g
	s.Start()
	var out run
	for s.Phase == core.Playing && s.Pieces < maxPieces {
		out.colours = append(out.colours, s.Cur.Colour)
		if d, ok := g.(*Director); ok {
			out.modes[d.Last.Mode]++
		}
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
		before := s.Clears
		s.HardDrop()
		for i := 0; i < 400 && !s.Settled; i++ {
			s.Tick()
			if s.Combo > 1 && s.FlashT == core.FlashTime {
				out.chains++
			}
		}
		if s.Clears > before {
			out.clearAt = append(out.clearAt, s.Pieces)
		}
	}
	out.pieces, out.clears, out.score, out.over = s.Pieces, s.Clears, s.Score, s.Phase == core.Over
	return out
}

func TestDirectorIsSeeded(t *testing.T) {
	a, b := bot(11, &Director{}, 120), bot(11, &Director{}, 120)
	if a.pieces != b.pieces || a.score != b.score || string(a.colours) != string(b.colours) {
		t.Fatalf("same seed, different runs: %d/%d pieces, %d/%d points", a.pieces, b.pieces, a.score, b.score)
	}
}

// The Director only reorders the bag: every aligned window of seven pieces
// still holds each shape exactly once.
func TestDirectorKeepsTheBagFair(t *testing.T) {
	s := core.NewSim(5, 4)
	d := &Director{}
	s.Gen = d
	s.Start()
	for y := s.GH / 2; y < s.GH; y++ {
		for x := 0; x < s.GW; x += 2 {
			s.Sand[y*s.GW+x] = uint8(1 + (x/9)%core.Colours)
		}
	}
	r := core.RNG(77)
	for len(s.Bag(&r)) != 7 {
		d.Next(s, &r)
	}
	for w := 0; w < 200; w++ {
		var seen [7]int
		for i := 0; i < 7; i++ {
			seen[d.Next(s, &r).Kind]++
		}
		for k, n := range seen {
			if n != 1 {
				t.Fatalf("window %d: shape %d dealt %d times", w, k, n)
			}
		}
	}
}

// Colours must look random to a player: balanced shares, repeats about as
// often as chance, and never long runs of one colour.
func TestDirectorColoursLookNatural(t *testing.T) {
	var shares [core.Colours + 1]int
	total, repeats, longest := 0, 0, 0
	for seed := uint64(1); seed <= 12; seed++ {
		cs := bot(seed, &Director{}, 400).colours
		runLen := 1
		for i, c := range cs {
			shares[c]++
			total++
			if i > 0 && c == cs[i-1] {
				repeats++
				runLen++
			} else {
				runLen = 1
			}
			longest = max(longest, runLen)
		}
	}
	for c := 1; c <= core.Colours; c++ {
		if f := float64(shares[c]) / float64(total); f < 0.15 || f > 0.36 {
			t.Errorf("colour %d is %.0f%% of pieces", c, 100*f)
		}
	}
	rep := float64(repeats) / float64(total)
	t.Logf("%d pieces: same colour as the previous %.0f%% (chance 25%%), longest run %d", total, 100*rep, longest)
	if rep > 0.32 {
		t.Errorf("a colour follows itself %.0f%% of the time", 100*rep)
	}
	if longest > 4 {
		t.Errorf("a run of %d pieces of one colour", longest)
	}
}

// The closer the stack is to the top, the more the Director helps.
func TestDirectorHelpsMoreNearTheTop(t *testing.T) {
	assistAt := func(fill float64) float64 {
		s := core.NewSim(3, 4)
		d := &Director{}
		s.Gen = d
		s.Start()
		for y := int(float64(s.GH) * (1 - fill)); y < s.GH; y++ {
			for x := 0; x < s.GW; x++ {
				if (x/5+y/7)%3 != 0 {
					s.Sand[y*s.GW+x] = uint8(1 + (x/13+y/11)%core.Colours)
				}
			}
		}
		r := core.RNG(9)
		d.Next(s, &r)
		return d.Last.Assist
	}
	low, high := assistAt(0.2), assistAt(0.8)
	t.Logf("assistance: stack at 20%% → %.2f, at 80%% → %.2f", low, high)
	if !(high > low+0.4) || low > calmCap {
		t.Fatalf("assistance %.2f low, %.2f high", low, high)
	}
}

// After a clear, another clear comes within the next three pieces more often
// with the Director than with blind dealing: chains and streaks are fed.
func TestDirectorFeedsChains(t *testing.T) {
	followRate := func(g func() core.Generator) (float64, int) {
		follow, clears := 0, 0
		for seed := uint64(1); seed <= 40; seed++ {
			at := bot(seed, g(), 400).clearAt
			for i, p := range at {
				clears++
				if i+1 < len(at) && at[i+1]-p <= 3 {
					follow++
				}
			}
		}
		return float64(follow) / float64(max(1, clears)), clears
	}
	blind, nb := followRate(func() core.Generator { return nil })
	dir, nd := followRate(func() core.Generator { return &Director{} })
	t.Logf("a clear followed by another within 3 pieces: blind %.0f%% of %d, director %.0f%% of %d", 100*blind, nb, 100*dir, nd)
	if dir <= blind {
		t.Fatalf("chains are not fed: %.2f vs blind %.2f", dir, blind)
	}
}

// The same player survives longer and clears more with the Director than
// with blind dealing, but the game still ends: helpful, not trivial.
func TestDirectorIsHelpfulButNotTrivial(t *testing.T) {
	if testing.Short() {
		t.Skip("plays many runs")
	}
	const runs, cap = 12, 600
	var blind, dir run
	ended := 0
	var modes [4]int
	for i := uint64(1); i <= runs; i++ {
		b := bot(i, nil, cap)
		blind.pieces += b.pieces
		blind.clears += b.clears
		d := bot(i, &Director{}, cap)
		dir.pieces += d.pieces
		dir.clears += d.clears
		dir.chains += d.chains
		if d.over {
			ended++
		}
		for m := range modes {
			modes[m] += d.modes[m]
		}
	}
	t.Logf("pieces: blind %d, director %d; clears: blind %d, director %d; chains %d; runs ended %d/%d",
		blind.pieces, dir.pieces, blind.clears, dir.clears, dir.chains, ended, runs)
	t.Logf("deals: blind %d, flow %d, chain %d, rescue %d", modes[Blind], modes[Flow], modes[Chain], modes[Rescue])
	if dir.pieces <= blind.pieces || dir.clears <= blind.clears {
		t.Fatal("the Director did not help")
	}
	if ended == 0 {
		t.Fatal("no run ended within the cap: too easy")
	}
	if modes[Blind] < (modes[Flow]+modes[Chain]+modes[Rescue])/3 {
		t.Fatal("almost every piece is steered: help would show")
	}
}

func BenchmarkDirectorNext(b *testing.B) {
	s := core.NewSim(1, core.MaxB)
	d := &Director{}
	s.Gen = d
	s.Start()
	for y := s.GH / 2; y < s.GH; y++ {
		for x := 0; x < s.GW; x++ {
			s.Sand[y*s.GW+x] = uint8(1 + (x/7+y/9)%core.Colours)
		}
	}
	r := core.RNG(3)
	b.ReportAllocs()
	for b.Loop() {
		d.Next(s, &r)
	}
}
