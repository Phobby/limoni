package gen

import (
	"math/bits"

	"github.com/thebanri/limoni/apps/lemondrop-mobile/blocks"
	"github.com/thebanri/limoni/apps/lemondrop-mobile/core"
)

// ZestProfile is how the placement mode deals its trays. Shown in settings.
type ZestProfile struct {
	Name string
	// Guarantee: every tray dealt can be placed completely, in some order,
	// with clears in between — whenever sampling finds such a tray or an
	// exhaustive search over the small pieces (≤3 cells) does.
	Guarantee bool
	// Crowd lowers the weight of big pieces (5+ cells) as the board fills:
	// weight × (1 − Crowd × fill).
	Crowd float64
	// Samples is how many solvable trays to compare when Prefer is set.
	Samples int
	// Prefer picks, among Samples solvable trays, the one with the most
	// lines a single placement could clear right now.
	Prefer bool
	// Budget bounds the search: nodes visited per deal.
	Budget int
}

var (
	ZestRelaxed   = ZestProfile{Name: "Relaxed", Guarantee: true, Crowd: 0.8, Samples: 6, Prefer: true, Budget: 60000}
	ZestStandard  = ZestProfile{Name: "Standard", Guarantee: true, Crowd: 0.4, Budget: 60000}
	ZestChallenge = ZestProfile{Name: "Challenge", Guarantee: false, Crowd: 0, Budget: 0}
)

// Zest implements blocks.Dealer.
type Zest struct {
	P ZestProfile

	w     []float64 // per shape, for this deal
	cand  []int
	nodes int
}

// Deal implements blocks.Dealer.
func (z *Zest) Deal(g *blocks.Game, r *core.RNG) [3]blocks.Slot {
	occ := g.Occ
	fill := float64(bits.OnesCount64(occ)) / 64
	z.cand = z.cand[:0]
	z.w = z.w[:0]
	total := 0.0
	for i, s := range blocks.Shapes {
		if !blocks.AnyFit(occ, s) {
			continue
		}
		w := baseWeight(s)
		if s.Cells >= 5 {
			w *= max(0.05, 1-z.P.Crowd*fill)
		}
		z.cand = append(z.cand, i)
		z.w = append(z.w, w)
		total += w
	}
	if len(z.cand) == 0 { // nothing fits: the run ends with whatever is dealt
		return z.colour(r, [3]int{0, 0, 0})
	}
	pick := func() int {
		x := r.Float() * total
		for k, w := range z.w {
			x -= w
			if x < 0 {
				return z.cand[k]
			}
		}
		return z.cand[len(z.cand)-1]
	}
	if !z.P.Guarantee {
		return z.colour(r, [3]int{pick(), pick(), pick()})
	}
	z.nodes = 0
	best, bestV, found := [3]int{}, -1, 0
	for attempt := 0; attempt < 64 && z.nodes < z.P.Budget; attempt++ {
		t := [3]int{pick(), pick(), pick()}
		if !z.solvable(occ, t) {
			continue
		}
		if !z.P.Prefer {
			return z.colour(r, t)
		}
		v := clearPotential(occ, t)*4 + r.Intn(4)
		if v > bestV {
			best, bestV = t, v
		}
		if found++; found >= z.P.Samples {
			break
		}
	}
	if found > 0 {
		return z.colour(r, best)
	}
	// Fallback: the smallest pieces, exhaustively, so a guarantee profile
	// only ends a run when no tray at all could be placed.
	small := smallShapes()
	for _, a := range small {
		for _, b := range small {
			for _, c := range small {
				t := [3]int{a, b, c}
				z.nodes = 0
				if z.solvable(occ, t) {
					return z.colour(r, t)
				}
			}
		}
	}
	return z.colour(r, [3]int{small[0], small[0], small[0]})
}

func (z *Zest) colour(r *core.RNG, t [3]int) (out [3]blocks.Slot) {
	for i, s := range t {
		out[i] = blocks.Slot{Shape: s, Colour: uint8(1 + r.Intn(blocks.Colours))}
	}
	return out
}

// baseWeight favours middle sizes; single cells are rare, 3×3 rarer.
func baseWeight(s blocks.Shape) float64 {
	switch {
	case s.Cells == 1:
		return 0.6
	case s.Cells >= 9:
		return 0.35
	case s.Cells >= 6:
		return 0.6
	}
	return 1
}

// solvable reports whether the three pieces can all be placed in some order,
// clearing between placements. The search is bounded by the profile's budget.
func (z *Zest) solvable(occ uint64, t [3]int) bool {
	perms := [6][3]int{{0, 1, 2}, {0, 2, 1}, {1, 0, 2}, {1, 2, 0}, {2, 0, 1}, {2, 1, 0}}
	for _, p := range perms {
		if z.dfs(occ, t[p[0]], t[p[1]], t[p[2]], 3) {
			return true
		}
	}
	return false
}

func (z *Zest) dfs(occ uint64, a, b, c, left int) bool {
	if left == 0 {
		return true
	}
	if z.P.Budget > 0 && z.nodes >= z.P.Budget {
		return false
	}
	s := blocks.Shapes[a]
	for r := 0; r+s.H <= blocks.N; r++ {
		for col := 0; col+s.W <= blocks.N; col++ {
			if occ&s.At(r, col) != 0 {
				continue
			}
			z.nodes++
			next, _ := blocks.PlaceOcc(occ, s, r, col)
			if z.dfs(next, b, c, -1, left-1) {
				return true
			}
		}
	}
	return false
}

// clearPotential is the most lines one of the pieces could clear right now.
func clearPotential(occ uint64, t [3]int) int {
	best := 0
	for _, i := range t {
		s := blocks.Shapes[i]
		for r := 0; r+s.H <= blocks.N; r++ {
			for c := 0; c+s.W <= blocks.N; c++ {
				if occ&s.At(r, c) == 0 {
					_, n := blocks.PlaceOcc(occ, s, r, c)
					best = max(best, n)
				}
			}
		}
	}
	return best
}

var small []int

func smallShapes() []int {
	if small == nil {
		for i, s := range blocks.Shapes {
			if s.Cells <= 3 {
				small = append(small, i)
			}
		}
	}
	return small
}
