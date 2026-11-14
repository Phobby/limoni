// Package gen deals Lemon Drop's pieces.
//
// The Director chooses each next piece's colour, and the order of shapes
// within the 7-bag, from how the run is going: how close the stack is to the
// top, how long since the last clear, whether a chain is under way, and how
// close each colour is to a wall-to-wall run. Near defeat it helps more;
// right after a clear it favours colours that can chain; on a comfortable
// board it mostly stays out of the way. It never deals to make the player
// lose, never changes how many of each shape come (every seven pieces still
// hold each shape once), and keeps colours looking natural (no long runs of
// one colour, balanced shares). See docs/ALGORITHMS.md.
package gen

import (
	"math"

	"github.com/thebanri/limoni/apps/lemondrop-mobile/core"
)

// Tuning. The tests in director_test.go pin down what these produce.
const (
	baseAssist   = 0.12 // chance of an evaluated (rather than blind) deal on a calm board
	dangerAssist = 0.68 // added as the stack reaches the top
	stallAssist  = 0.20 // added after ~10 pieces without a clear
	chainAssist  = 0.50 // added right after a clear, fading over a few pieces
	calmCap      = 0.55 // assistance never exceeds this on a low stack…
	dangerCap    = 0.90 // …and this on a high one
)

// Mode is what the Director was doing for the last deal (for tests and a
// debug overlay; never shown to players).
type Mode uint8

const (
	Blind  Mode = iota // dealt like the classic bag
	Flow               // evaluated, light touch
	Chain              // feeding a chain after a clear
	Rescue             // helping out of danger
)

// Director implements core.Generator.
type Director struct {
	h       [core.MaxGW]int                    // surface height per grain column (row of the top grain)
	near    [core.Colours + 1][core.MaxGW]bool // colour present near the surface
	recent  [8]uint8                           // last colours dealt
	nrec    int
	clears  int     // s.Clears when last asked
	pieces  int     // s.Pieces when last asked
	calm    int     // pieces since the last clear
	heat    float64 // recent clearing, decays per piece
	weights [7 * core.Colours]float64

	// Last describes the last deal.
	Last struct {
		Mode           Mode
		Assist, Danger float64
	}
}

// Next implements core.Generator.
func (d *Director) Next(s *core.Sim, r *core.RNG) core.Piece {
	bag := s.Bag(r)
	if s.Phase != core.Playing {
		k := s.DealBag(len(bag) - 1)
		return core.Piece{Kind: k, Colour: uint8(1 + r.Intn(core.Colours))}
	}
	d.observe(s)
	d.survey(s)
	d.predict(s, s.Next) // the piece about to fall lands before this one does
	danger := smooth(0.35, 0.82, d.topFrac(s))

	chain := d.heat
	stall := min(1, float64(d.calm)/10)
	assist := baseAssist + dangerAssist*danger + stallAssist*stall + chainAssist*chain
	assist = min(assist, calmCap+(dangerCap-calmCap)*danger)
	d.Last.Assist, d.Last.Danger = assist, danger

	if r.Float() >= assist {
		d.Last.Mode = Blind
		k := s.DealBag(len(bag) - 1)
		return d.deal(k, d.naturalColour(r))
	}

	mode := Flow
	switch {
	case danger > 0.55:
		mode = Rescue
	case chain > 0.25:
		mode = Chain
	}
	d.Last.Mode = mode

	// Value every (shape in the bag, colour) by its best landing.
	best := math.Inf(-1)
	n := len(bag) * core.Colours
	for bi, k := range bag {
		for c := 1; c <= core.Colours; c++ {
			v := d.value(s, k, uint8(c), danger, chain) + d.naturalness(uint8(c), danger)
			d.weights[bi*core.Colours+c-1] = v
			best = max(best, v)
		}
	}
	// Softmax: decisive when rescuing, loose otherwise, so help reads as luck.
	temp := 0.22 - 0.10*danger
	sum := 0.0
	for i := 0; i < n; i++ {
		w := math.Exp((d.weights[i] - best) / temp)
		d.weights[i] = w
		sum += w
	}
	x := r.Float() * sum
	pick := n - 1
	for i := 0; i < n; i++ {
		if x -= d.weights[i]; x < 0 {
			pick = i
			break
		}
	}
	k := s.DealBag(pick / core.Colours)
	return d.deal(k, uint8(1+pick%core.Colours))
}

func (d *Director) deal(kind int, c uint8) core.Piece {
	d.recent[d.nrec%len(d.recent)] = c
	d.nrec++
	return core.Piece{Kind: kind, Colour: c}
}

// observe updates the run's momentum from the score so far.
func (d *Director) observe(s *core.Sim) {
	if s.Pieces < d.pieces || s.Clears < d.clears { // a new run
		*d = Director{}
	}
	if s.Pieces == d.pieces && s.Clears == d.clears {
		return
	}
	newClears := s.Clears - d.clears
	d.clears, d.pieces = s.Clears, s.Pieces
	if newClears > 0 {
		d.calm = 0
		d.heat = min(1, 0.5*d.heat+0.6+0.2*float64(newClears-1))
	} else {
		d.calm++
		d.heat *= 0.6
	}
}

// survey fills the height map and the near-surface colour table.
func (d *Director) survey(s *core.Sim) {
	gw, gh, depth := s.GW, s.GH, 3*s.B
	for x := 0; x < gw; x++ {
		h := gh
		for y := 0; y < gh; y++ {
			if s.Sand[y*gw+x] != 0 {
				h = y
				break
			}
		}
		d.h[x] = h
		for c := range d.near {
			d.near[c][x] = false
		}
		for y := h; y < min(gh, h+depth); y++ {
			d.near[s.Sand[y*gw+x]&core.ColourMask][x] = true
		}
	}
}

func (d *Director) topFrac(s *core.Sim) float64 {
	top := s.GH
	for x := 0; x < s.GW; x++ {
		top = min(top, d.h[x])
	}
	return 1 - float64(top)/float64(s.GH)
}

// landing is where a shape comes to rest on the height map at grain column x.
func (d *Director) landing(s *core.Sim, kind, rot, x int) int {
	b := s.B
	y := s.GH
	for _, c := range core.Shapes[kind][rot] {
		bot := (int(c[1]) + 1) * b
		for gx := x + int(c[0])*b; gx < x+int(c[0])*b+b; gx++ {
			y = min(y, d.h[gx]-bot)
		}
	}
	return y
}

// placements calls f for every rotation and half-block position.
func placements(s *core.Sim, kind int, f func(rot, x int)) {
	b := s.B
	for rot := 0; rot < 4; rot++ {
		minX, maxX := 4, 0
		for _, c := range core.Shapes[kind][rot] {
			minX, maxX = min(minX, int(c[0])), max(maxX, int(c[0]))
		}
		for x := -minX * b; x+(maxX+1)*b <= s.GW; x += s.Step() {
			f(rot, x)
		}
	}
}

// missing is how many grain columns lack colour c near the surface.
func (d *Director) missing(gw int, c uint8) int {
	n := 0
	for x := 0; x < gw; x++ {
		if !d.near[c][x] {
			n++
		}
	}
	return n
}

// value judges a shape and colour by its best landing: how far it brings its
// colour towards a wall-to-wall band (a clear), how much that matters now
// (danger: get a clear; chain: keep clearing), against stacking high and
// leaving gaps under it.
func (d *Director) value(s *core.Sim, kind int, c uint8, danger, chain float64) float64 {
	b, gw, gh := s.B, s.GW, float64(s.GH)
	before := d.missing(gw, c)
	best := math.Inf(-1)
	placements(s, kind, func(rot, x int) {
		y := d.landing(s, kind, rot, x)
		if y+4*b < 0 {
			return
		}
		covered, gaps, topY := 0, 0, s.GH
		for _, cell := range core.Shapes[kind][rot] {
			bx, by := x+int(cell[0])*b, y+int(cell[1])*b
			topY = min(topY, by)
			below := false
			for _, o := range core.Shapes[kind][rot] {
				below = below || o[0] == cell[0] && o[1] == cell[1]+1
			}
			for gx := bx; gx < bx+b; gx++ {
				if !d.near[c][gx] {
					covered++
				}
				if u := d.h[gx] - (by + b); u > 0 && !below {
					gaps += u
				}
			}
		}
		after := before - covered
		progress := float64(covered) / float64(gw)
		complete := 0.0
		switch {
		case after <= 0:
			complete = 1 // this colour likely spans wall to wall: a clear
		case after <= b:
			complete = 0.4 // one block short
		}
		closeness := 1 - float64(max(after, 0))/float64(gw)
		height := 1 - float64(topY)/gh
		v := (0.7+0.9*danger)*progress +
			(0.5+1.1*danger+0.9*chain)*complete +
			0.6*chain*closeness -
			(0.5+1.6*danger)*height*height -
			(0.2+0.4*danger)*float64(gaps)/float64(b*b*4)
		best = max(best, v)
	})
	return best
}

// predict lays the piece about to fall on the height map where a sensible
// player would put it, so the next deal is judged on the board it will meet.
func (d *Director) predict(s *core.Sim, p core.Piece) {
	b := s.B
	bestV, bestRot, bestX := math.Inf(-1), 0, 0
	placements(s, p.Kind, func(rot, x int) {
		y := d.landing(s, p.Kind, rot, x)
		v := -float64(s.GH-y) * 0.02
		for _, cell := range core.Shapes[p.Kind][rot] {
			bx := x + int(cell[0])*b
			for gx := bx; gx < bx+b; gx++ {
				if !d.near[p.Colour][gx] {
					v += 0.05
				}
			}
		}
		if v > bestV {
			bestV, bestRot, bestX = v, rot, x
		}
	})
	y := d.landing(s, p.Kind, bestRot, bestX)
	for _, cell := range core.Shapes[p.Kind][bestRot] {
		bx, by := bestX+int(cell[0])*b, y+int(cell[1])*b
		for gx := bx; gx < bx+b; gx++ {
			if by < d.h[gx] {
				d.h[gx] = max(by, 0)
			}
			for c := range d.near {
				d.near[c][gx] = false
			}
			d.near[p.Colour][gx] = true
		}
	}
}

// naturalness keeps colours looking random: a third piece of one colour in a
// row is all but ruled out, a second is discouraged a little, and a colour
// over its share of the last eight is pushed back. These hold in a rescue
// too, so help never shows as a streak of one colour.
func (d *Director) naturalness(c uint8, danger float64) float64 {
	n := min(d.nrec, len(d.recent))
	if n == 0 {
		return 0
	}
	last := func(i int) uint8 { return d.recent[(d.nrec-1-i)%len(d.recent)] }
	v := 0.0
	if n >= 2 && last(0) == c && last(1) == c {
		v -= 5
	} else if last(0) == c {
		v -= 0.18 * (1 - 0.4*danger)
	}
	same := 0
	for i := 0; i < n; i++ {
		if last(i) == c {
			same++
		}
	}
	if share := float64(same) / float64(n); share > 0.375 {
		v -= (share - 0.375) * 2
	}
	return v
}

// naturalColour is a blind deal's colour: uniform, but never a third in a row.
func (d *Director) naturalColour(r *core.RNG) uint8 {
	c := uint8(1 + r.Intn(core.Colours))
	n := min(d.nrec, len(d.recent))
	if n >= 2 {
		a, b := d.recent[(d.nrec-1)%len(d.recent)], d.recent[(d.nrec-2)%len(d.recent)]
		if a == c && b == c {
			c = uint8(1 + (int(c)+r.Intn(core.Colours-1))%core.Colours)
		}
	}
	return c
}

func smooth(a, b, x float64) float64 {
	t := max(0, min(1, (x-a)/(b-a)))
	return t * t * (3 - 2*t)
}
