// Package gen deals pieces for both modes.
//
// FairDrop (falling-sand mode) and Zest (placement mode) are written for
// this game. They use ideas documented in the Block Blast research reports
// (reports/piece-generation.md in the research workspace): a seeded LCG-style
// RNG, board-aware filtering, bounded look-ahead with a time/node budget and
// a guaranteed-playable fallback. They deliberately leave out the research's
// "reward rationing" families (avoid board clears, break clearing streaks,
// hidden hard modes): every profile here is visible to the player and its
// effect is measurable by the tests. See docs/ALGORITHMS.md.
package gen

import (
	"math"

	"github.com/thebanri/limoni/apps/lemondrop-mobile/core"
)

// DropProfile is how much FairDrop helps. All numbers are shown in the game's
// settings screen and checked by tests.
type DropProfile struct {
	Name string
	// Assist is the chance, at full danger, that a piece is chosen by the
	// evaluator instead of dealt blindly from the bag.
	Assist float64
	// Calm is the chance on a low stack (danger 0); assistance ramps
	// linearly from Calm to Assist as the stack rises to DangerAt.
	Calm, DangerAt float64
	// Temperature spreads the evaluator's choice over good candidates
	// rather than always the best (0 would be deterministic best).
	Temperature float64
}

var (
	Relaxed  = DropProfile{Name: "Relaxed", Assist: 0.85, Calm: 0.35, DangerAt: 0.55, Temperature: 0.08}
	Balanced = DropProfile{Name: "Balanced", Assist: 0.5, Calm: 0.1, DangerAt: 0.7, Temperature: 0.15}
)

// FairDrop keeps the 7-bag's fairness (every seven pieces hold each shape
// exactly once) and only chooses, when it assists, which shape of the bag
// comes next and its colour.
type FairDrop struct {
	P DropProfile

	h      [core.MaxGW]int // column heights in grains (index of top grain; GH when empty)
	near   [core.Colours + 1][core.MaxGW]bool
	scores [7 * core.Colours]float64
}

// Next implements core.Generator.
func (f *FairDrop) Next(s *core.Sim, r *core.RNG) core.Piece {
	bag := s.Bag(r)
	danger := f.survey(s)
	t := min(1, danger/max(f.P.DangerAt, 1e-9))
	assist := f.P.Calm + (f.P.Assist-f.P.Calm)*t
	if s.Phase != core.Playing || r.Float() >= assist {
		// Blind deal, exactly as the classic bag does.
		k := s.DealBag(len(bag) - 1)
		return core.Piece{Kind: k, Colour: uint8(1 + r.Intn(core.Colours))}
	}
	// Score every (shape in the bag, colour) by its best landing.
	best := math.Inf(-1)
	for bi, k := range bag {
		for c := 1; c <= core.Colours; c++ {
			v := f.value(s, k, uint8(c))
			f.scores[bi*core.Colours+c-1] = v
			best = max(best, v)
		}
	}
	// Choose by softmax over the candidates (temperature in value units).
	sum := 0.0
	for i := 0; i < len(bag)*core.Colours; i++ {
		w := math.Exp((f.scores[i] - best) / f.P.Temperature)
		f.scores[i] = w
		sum += w
	}
	x := r.Float() * sum
	pick := len(bag)*core.Colours - 1
	for i := 0; i < len(bag)*core.Colours; i++ {
		x -= f.scores[i]
		if x < 0 {
			pick = i
			break
		}
	}
	k := s.DealBag(pick / core.Colours)
	return core.Piece{Kind: k, Colour: uint8(1 + pick%core.Colours)}
}

// survey fills the height map and the "colour near the surface" table and
// returns how dangerous the stack is, 0 (empty) … 1 (at the top).
func (f *FairDrop) survey(s *core.Sim) float64 {
	gw, gh, depth := s.GW, s.GH, 3*s.B
	top := gh
	for x := 0; x < gw; x++ {
		h := gh
		for y := 0; y < gh; y++ {
			if s.Sand[y*gw+x] != 0 {
				h = y
				break
			}
		}
		f.h[x] = h
		top = min(top, h)
		for c := range f.near {
			f.near[c][x] = false
		}
		for y := h; y < min(gh, h+depth); y++ {
			f.near[s.Sand[y*gw+x]&core.ColourMask][x] = true
		}
	}
	return 1 - float64(top)/float64(gh)
}

// value is a shape-and-colour's best landing, judged on the height map: how
// much it widens a band of its colour near the surface (what a wall-to-wall
// run needs), minus how high it stacks and the gaps it leaves under itself.
// Sand slumps after landing, so this is an estimate; it is bounded to
// 4 rotations × (GW / half a block) positions.
func (f *FairDrop) value(s *core.Sim, kind int, colour uint8) float64 {
	b, gw, gh := s.B, s.GW, s.GH
	best := math.Inf(-1)
	for rot := 0; rot < 4; rot++ {
		cells := &core.Shapes[kind][rot]
		minX, maxX := 4, 0
		for _, c := range cells {
			minX = min(minX, int(c[0]))
			maxX = max(maxX, int(c[0]))
		}
		for x := -minX * b; x+(maxX+1)*b <= gw; x += s.Step() {
			// Landing: the piece's lowest grain in each column rests on the height map.
			y := gh
			for _, c := range cells {
				bot := (int(c[1]) + 1) * b
				for gx := x + int(c[0])*b; gx < x+int(c[0])*b+b; gx++ {
					y = min(y, f.h[gx]-bot)
				}
			}
			if y+4*b < 0 {
				continue
			}
			topY, gaps, newCols := gh, 0, 0
			for _, c := range cells {
				bx := x + int(c[0])*b
				by := y + int(c[1])*b
				topY = min(topY, by)
				for gx := bx; gx < bx+b; gx++ {
					// Grains below this block that are empty until the stack.
					if under := f.h[gx] - (by + b); under > 0 && !occupiedBelow(cells, c, b) {
						gaps += under
					}
					if !f.near[colour][gx] {
						newCols++
					}
				}
			}
			height := 1 - float64(topY)/float64(gh)
			band := 0
			for gx := 0; gx < gw; gx++ {
				if f.near[colour][gx] {
					band++
				}
			}
			widen := float64(min(newCols/b, gw/b-band/b)) / float64(core.Cols)
			v := 0.6*float64(band)/float64(gw)*0.5 + 0.9*widen - 1.4*height*height - 0.25*float64(gaps)/float64(b*b*4)
			best = max(best, v)
		}
	}
	return best
}

// occupiedBelow reports whether the piece itself has a block right under c.
func occupiedBelow(cells *[4][2]int8, c [2]int8, b int) bool {
	for _, o := range cells {
		if o[0] == c[0] && o[1] == c[1]+1 {
			return true
		}
	}
	return false
}
