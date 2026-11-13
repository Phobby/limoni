package gen

import (
	"math/rand/v2"
	"testing"
	"time"

	"github.com/thebanri/limoni/apps/lemondrop-mobile/blocks"
	"github.com/thebanri/limoni/apps/lemondrop-mobile/core"
)

// randomBoard fills cells at random to about fill, avoiding full lines.
func randomBoard(seed uint64, fill float64) uint64 {
	r := rand.New(rand.NewPCG(seed, seed^0x9e37))
	var occ uint64
	for i := 0; i < 64; i++ {
		if r.Float64() < fill {
			occ |= 1 << i
		}
	}
	rows, cols := blocks.Full(occ)
	return occ &^ blocks.ClearMask(rows, cols)
}

func TestZestIsSeeded(t *testing.T) {
	deal := func() [3]blocks.Slot {
		g := &blocks.Game{}
		g.Occ = randomBoard(4, 0.5)
		r := core.RNG(123)
		return (&Zest{P: ZestStandard}).Deal(g, &r)
	}
	if a, b := deal(), deal(); a != b {
		t.Fatalf("same seed, different trays: %v %v", a, b)
	}
}

// Guarantee profiles deal a tray that can be placed completely whenever a
// tray of small pieces could be; checked by an independent exhaustive search.
func TestZestGuaranteeHolds(t *testing.T) {
	for _, p := range []ZestProfile{ZestStandard, ZestRelaxed} {
		z := &Zest{P: p}
		ref := &Zest{P: ZestProfile{Guarantee: true}} // unbounded search
		dealt, possible := 0, 0
		for seed := uint64(1); seed <= 400; seed++ {
			fill := 0.25 + 0.6*float64(seed%10)/10
			g := &blocks.Game{}
			g.Occ = randomBoard(seed, fill)
			r := core.RNG(seed)
			tray := z.Deal(g, &r)
			anySmall := false
			for _, a := range smallShapes() {
				if ref.solvable(g.Occ, [3]int{a, a, a}) {
					anySmall = true
					break
				}
			}
			if !anySmall {
				continue
			}
			possible++
			if ref.solvable(g.Occ, [3]int{tray[0].Shape, tray[1].Shape, tray[2].Shape}) {
				dealt++
			}
		}
		if dealt != possible {
			t.Fatalf("%s: %d of %d solvable boards got a solvable tray", p.Name, dealt, possible)
		}
		t.Logf("%s: %d/%d boards got a solvable tray", p.Name, dealt, possible)
	}
}

// Relaxed shifts weight away from big pieces as the board fills; Challenge
// does not. Measured, not assumed.
func TestZestProfilesDifferMeasurably(t *testing.T) {
	bigShare := func(p ZestProfile) float64 {
		z := &Zest{P: p}
		big, n := 0, 0
		for seed := uint64(1); seed <= 300; seed++ {
			g := &blocks.Game{}
			g.Occ = randomBoard(seed, 0.55)
			r := core.RNG(seed)
			for _, s := range z.Deal(g, &r) {
				if blocks.Shapes[s.Shape].Cells >= 5 {
					big++
				}
				n++
			}
		}
		return float64(big) / float64(n)
	}
	relaxed, challenge := bigShare(ZestRelaxed), bigShare(ZestChallenge)
	t.Logf("share of 5+ cell pieces on 55%% boards: relaxed %.2f, challenge %.2f", relaxed, challenge)
	if relaxed >= challenge {
		t.Fatalf("relaxed (%.2f) should deal fewer big pieces than challenge (%.2f)", relaxed, challenge)
	}
}

func TestZestDealIsBounded(t *testing.T) {
	z := &Zest{P: ZestRelaxed}
	worst := time.Duration(0)
	for seed := uint64(1); seed <= 200; seed++ {
		g := &blocks.Game{}
		g.Occ = randomBoard(seed, 0.7)
		r := core.RNG(seed)
		t0 := time.Now()
		z.Deal(g, &r)
		worst = max(worst, time.Since(t0))
	}
	t.Logf("slowest deal: %v", worst)
	if worst > 250*time.Millisecond {
		t.Fatalf("a deal took %v", worst)
	}
}

func BenchmarkZestDeal(b *testing.B) {
	z := &Zest{P: ZestRelaxed}
	g := &blocks.Game{}
	g.Occ = randomBoard(9, 0.5)
	r := core.RNG(1)
	for b.Loop() {
		z.Deal(g, &r)
	}
}
