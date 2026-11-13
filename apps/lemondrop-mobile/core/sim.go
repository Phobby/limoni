// Package core holds Lemon Drop's rules: falling blocks that land as sand,
// and the colour run from wall to wall that clears.
//
// The rules, constants and random-number use are ported unchanged from the
// terminal game (apps/lemondrop/game.go); only the user interface, sound and
// rendering state were taken out. A step allocates nothing: everything lives
// in fixed arrays inside one Sim made at start-up. Visual and audio feedback
// is reported through Events and never feeds back into the rules, so the same
// inputs and seed give the same run and score at any frame rate.
package core

const (
	Cols = 10 // board width, in blocks
	Rows = 18 // board height, in blocks

	MinB = 2 // grains per block edge accepted by the shared scoreboard…
	MaxB = 6 // …and the most (apps/scoreboard/board/drop.go)

	MaxGW    = Cols * MaxB
	MaxGH    = Rows * MaxB
	MaxCells = MaxGW * MaxGH

	Colours = 4

	Hz = 60 // simulation steps a second
	Dt = 1.0 / Hz

	LockDelay = 0.4  // seconds a piece rests before it becomes sand
	MaxResets = 12   // moves that may restart that wait, per piece
	FlashTime = 0.45 // seconds a clear flashes before it goes
	SoftSpeed = 24.0 // blocks a second while soft-dropping

	MaxLog = 5000 // clears kept for the leaderboard log
)

// A grain is one byte: the colour in the low three bits (0 is empty), the
// shade in the next two, and ClearBit while it flashes.
const (
	ColourMask = 0x07
	ShadeShift = 3
	ClearBit   = 0x80
)

// Phase is where a run is.
type Phase uint8

const (
	Attract Phase = iota // playing by itself (menus' background)
	Playing
	Over
)

// Piece is the falling piece: Kind 0–6 (I J L O S T Z), Rot 0–3, Colour 1–4,
// X/Y the top-left of its 4×4 box in grains (Y may be negative), Fall the
// fraction of a grain fallen since the last whole one.
type Piece struct {
	Kind, Rot int
	Colour    uint8
	X, Y      int
	Fall      float64
}

// Generator deals pieces. Classic (the original 7-bag) is built in; others
// live in package gen. Next may read the Sim but must not change it, and
// must draw randomness only from the rng it is given so runs stay seeded.
type Generator interface {
	Next(s *Sim, rng *RNG) Piece
}

// Sim is one board and its run.
type Sim struct {
	Phase Phase

	B      int // grains per block edge
	GW, GH int // board size in grains

	Sand    [MaxCells]uint8
	vel     [MaxCells]uint8 // a grain's speed down, in 1/32 grain a step
	frac    [MaxCells]uint8 // progress towards its next cell, in 1/32
	awake   [MaxGH]bool
	Settled bool

	mark   [MaxCells]uint32
	stamp  uint32
	stack  [MaxCells]int32
	clr    [MaxCells]int32
	nclr   int
	FlashT float64 // time left on the current flash; 0 when none

	Cur, Next Piece
	bag       [7]int
	nbag      int
	resting   float64
	resets    int
	dropping  bool
	SoftDrop  bool

	Gen Generator // nil: the classic bag

	Score, Clears, Level, Combo int
	Pieces, DropPts             int
	ClearLog                    [MaxLog][2]int32
	NLog                        int
	GrainsMoved                 int // grains that moved on the last pass

	Ticks    uint64
	Now      float64
	acc      float64
	attractT float64
	rng      RNG

	ev   [64]Event
	nev  int
	evLo int
}

// NewSim makes a board at b grains to a block (MinB…MaxB), seeded.
func NewSim(seed uint64, b int) *Sim {
	s := &Sim{rng: RNG(seed | 1), Level: 1}
	s.SetSize(b)
	return s
}

// RNG is xorshift64*, as in the terminal game.
type RNG uint64

func (r *RNG) Next() uint64 {
	x := uint64(*r)
	x ^= x >> 12
	x ^= x << 25
	x ^= x >> 27
	*r = RNG(x)
	return x * 2685821657736338717
}

// Intn is in [0, n).
func (r *RNG) Intn(n int) int { return int(r.Next() >> 33 % uint64(n)) }

// Float is in [0, 1).
func (r *RNG) Float() float64 { return float64(r.Next()>>11) / (1 << 53) }

func (s *Sim) rnd() uint64    { return s.rng.Next() }
func (s *Sim) intn(n int) int { return s.rng.Intn(n) }
func (s *Sim) RNGState() RNG  { return s.rng }
func (s *Sim) SetRNG(r RNG)   { s.rng = r | 1 }

// SetSize empties the board and sizes it at b grains to a block.
func (s *Sim) SetSize(b int) {
	b = max(MinB, min(MaxB, b))
	s.B, s.GW, s.GH = b, Cols*b, Rows*b
	clear(s.Sand[:])
	clear(s.vel[:])
	clear(s.frac[:])
	clear(s.awake[:])
	s.nclr, s.FlashT, s.Settled = 0, 0, true
}

func (s *Sim) wake(y0, y1 int) {
	for y := max(y0, 0); y <= min(y1, s.GH-1); y++ {
		s.awake[y] = true
	}
	s.Settled = false
}

// WakeAll has the next pass visit every row: for a board changed by hand.
func (s *Sim) WakeAll() { s.wake(0, s.GH-1) }

// Start begins a run on an empty board.
func (s *Sim) Start() {
	s.SetSize(s.B)
	s.Phase = Playing
	s.Score, s.Clears, s.Level, s.Combo = 0, 0, 1, 0
	s.Pieces, s.DropPts, s.NLog = 0, 0, 0
	s.SoftDrop = false
	s.nbag = 0
	s.nev, s.evLo = 0, 0
	s.Next = s.draw()
	s.spawn()
	s.emit(Event{Kind: EvStart})
}

// Shapes holds each piece's four blocks in every rotation, as (x, y) in a
// 4×4 box; made by init from the spawn orientation.
var Shapes [7][4][4][2]int8

var spawnShapes = [7]struct {
	n     int8
	cells [4][2]int8
}{
	{4, [4][2]int8{{0, 1}, {1, 1}, {2, 1}, {3, 1}}}, // I
	{3, [4][2]int8{{0, 0}, {0, 1}, {1, 1}, {2, 1}}}, // J
	{3, [4][2]int8{{2, 0}, {0, 1}, {1, 1}, {2, 1}}}, // L
	{2, [4][2]int8{{0, 0}, {1, 0}, {0, 1}, {1, 1}}}, // O
	{3, [4][2]int8{{1, 0}, {2, 0}, {0, 1}, {1, 1}}}, // S
	{3, [4][2]int8{{1, 0}, {0, 1}, {1, 1}, {2, 1}}}, // T
	{3, [4][2]int8{{0, 0}, {1, 0}, {1, 1}, {2, 1}}}, // Z
}

func init() {
	for k, sh := range spawnShapes {
		cells := sh.cells
		for r := 0; r < 4; r++ {
			Shapes[k][r] = cells
			for i, c := range cells { // a quarter turn clockwise
				cells[i] = [2]int8{sh.n - 1 - c[1], c[0]}
			}
		}
	}
}

// kicks are the nudges a turn tries, in half blocks: sideways, then up.
var kicks = [...][2]int8{
	{0, 0}, {-1, 0}, {1, 0}, {-2, 0}, {2, 0}, {-4, 0}, {4, 0},
	{0, -1}, {-1, -1}, {1, -1}, {0, -2},
}

// draw deals the next piece: the generator's, or the classic bag of all
// seven shuffled, in one of the colours at random.
func (s *Sim) draw() Piece {
	if s.Gen != nil {
		return s.Gen.Next(s, &s.rng)
	}
	return s.ClassicNext(&s.rng)
}

// ClassicNext is the original deal: a shuffled bag of the seven shapes, a
// colour at random. Other generators may fall back to it.
func (s *Sim) ClassicNext(r *RNG) Piece {
	if s.nbag == 0 {
		for i := range s.bag {
			s.bag[i] = i
		}
		for i := len(s.bag) - 1; i > 0; i-- {
			j := r.Intn(i + 1)
			s.bag[i], s.bag[j] = s.bag[j], s.bag[i]
		}
		s.nbag = len(s.bag)
	}
	s.nbag--
	return Piece{Kind: s.bag[s.nbag], Colour: uint8(1 + r.Intn(Colours))}
}

// Bag is the shapes left in the current bag, refilled (shuffled with r)
// when empty. Generators that keep bag fairness pick from it with DealBag.
func (s *Sim) Bag(r *RNG) []int {
	if s.nbag == 0 {
		for i := range s.bag {
			s.bag[i] = i
		}
		for i := len(s.bag) - 1; i > 0; i-- {
			j := r.Intn(i + 1)
			s.bag[i], s.bag[j] = s.bag[j], s.bag[i]
		}
		s.nbag = len(s.bag)
	}
	return s.bag[:s.nbag]
}

// DealBag takes the shape at index i of Bag and returns it.
func (s *Sim) DealBag(i int) int {
	k := s.bag[i]
	s.bag[i] = s.bag[s.nbag-1]
	s.nbag--
	return k
}

// spawn puts the next piece at the top, centred; false if it does not fit.
func (s *Sim) spawn() bool {
	p := s.Next
	s.Next = s.draw()
	p = s.placeAtTop(p)
	s.Cur = p
	s.resting, s.resets = 0, 0
	s.emit(Event{Kind: EvSpawn})
	return s.Fits(p.Kind, p.Rot, p.X, p.Y)
}

// placeAtTop is where a piece enters: rotation 0, centred, its top row at
// the board's top edge.
func (s *Sim) placeAtTop(p Piece) Piece {
	top, wide := int8(4), int8(0)
	for _, c := range Shapes[p.Kind][0] {
		top = min(top, c[1])
		wide = max(wide, c[0]+1)
	}
	p.Rot, p.Fall = 0, 0
	p.X = (s.GW - int(wide)*s.B) / 2
	p.Y = -int(top) * s.B
	return p
}

// Fits reports whether a piece there overlaps neither a wall, the floor nor
// a grain. Above the top is open.
func (s *Sim) Fits(kind, rot, x, y int) bool {
	b := s.B
	for _, c := range Shapes[kind][rot] {
		x0, y0 := x+int(c[0])*b, y+int(c[1])*b
		if x0 < 0 || x0+b > s.GW || y0+b > s.GH {
			return false
		}
		for yy := max(y0, 0); yy < y0+b; yy++ {
			row := s.Sand[yy*s.GW+x0 : yy*s.GW+x0+b]
			for _, v := range row {
				if v != 0 {
					return false
				}
			}
		}
	}
	return true
}

// DropY is where the current piece would land, straight down.
func (s *Sim) DropY() int { return s.dropYFor(s.Cur.Kind, s.Cur.Rot, s.Cur.X, s.Cur.Y) }

func (s *Sim) dropYFor(kind, rot, x, y int) int {
	for s.Fits(kind, rot, x, y+1) {
		y++
	}
	return y
}

// Step is a sideways move's size in grains: half a block.
func (s *Sim) Step() int { return max(1, s.B/2) }

// Shift moves the piece sideways by dx grains, if it fits.
func (s *Sim) Shift(dx int) bool {
	p := &s.Cur
	if s.Phase != Playing || !s.Fits(p.Kind, p.Rot, p.X+dx, p.Y) {
		return false
	}
	p.X += dx
	s.moved()
	s.emit(Event{Kind: EvMove})
	return true
}

// Turn rotates the piece a quarter (1 clockwise, -1 back), kicking off a
// wall or pile when it has to.
func (s *Sim) Turn(dir int) bool {
	p := &s.Cur
	if s.Phase != Playing {
		return false
	}
	r := (p.Rot + dir + 4) % 4
	for _, k := range kicks {
		x, y := p.X+int(k[0])*s.B/2, p.Y+int(k[1])*s.B/2
		if s.Fits(p.Kind, r, x, y) {
			p.Rot, p.X, p.Y = r, x, y
			s.moved()
			s.emit(Event{Kind: EvTurn})
			return true
		}
	}
	return false
}

func (s *Sim) moved() {
	if s.resting > 0 && s.resets < MaxResets {
		s.resting = 0
		s.resets++
	}
}

// HardDrop puts the piece where it would land and makes it sand at once.
func (s *Sim) HardDrop() {
	if s.Phase != Playing {
		return
	}
	y := s.DropY()
	pts := (y - s.Cur.Y) / s.B * 2
	s.Score += pts
	s.DropPts += pts
	s.Cur.Y = y
	s.dropping = true
	s.lock()
}

// ShadeAt is a falling block's shading at a grain inside it: lit on the top
// and left edges, dark on the bottom and right, speckled inside.
func ShadeAt(lx, ly, b int) uint8 {
	switch {
	case lx == 0 || ly == 0:
		return 3
	case lx == b-1 || ly == b-1:
		return 0
	}
	return 1 + uint8((lx*5+ly*3)%3&1)
}

const (
	gravity   = 1 // speed gained a step, in 1/32 grain
	landSpeed = 4 // the speed a landed piece's grains start at
)

// sandShade is a grain's shade from three random bits.
var sandShade = [8]uint8{0, 1, 1, 1, 2, 2, 2, 3}

// lock turns the piece into sand and deals the next one.
func (s *Sim) lock() {
	p := &s.Cur
	b := s.B
	for _, c := range Shapes[p.Kind][p.Rot] {
		x0, y0 := p.X+int(c[0])*b, p.Y+int(c[1])*b
		for ly := 0; ly < b; ly++ {
			y := y0 + ly
			if y < 0 {
				continue
			}
			for lx := 0; lx < b; lx++ {
				i := y*s.GW + x0 + lx
				if s.Sand[i] != 0 {
					continue
				}
				r := s.rnd()
				s.Sand[i] = p.Colour | sandShade[r&7]<<ShadeShift
				if y < s.GH-1 {
					s.vel[i] = landSpeed
				}
				s.frac[i] = uint8(r>>8) & 31
			}
		}
	}
	s.wake(p.Y, p.Y+4*b-1)
	s.Combo = 0
	if s.Phase != Playing {
		return
	}
	s.Pieces++
	k := EvLand
	if s.dropping {
		k = EvDrop
	}
	s.emit(Event{Kind: k, X: p.X, Y: p.Y, Colour: p.Colour, Piece: p.Kind, Rot: p.Rot})
	s.dropping = false
	if !s.spawn() {
		s.Phase = Over
		s.SoftDrop = false
		s.emit(Event{Kind: EvGameOver})
	}
}

// fallSpeed is how fast pieces fall at a level, in blocks a second.
func FallSpeed(level int) float64 {
	v := 1.2
	for i := 1; i < level; i++ {
		v *= 1.2
	}
	return min(v, 20)
}

// Update advances by elapsed seconds in fixed steps; it returns the steps run.
func (s *Sim) Update(elapsed float64) int {
	s.acc += min(elapsed, 0.1)
	n := 0
	for s.acc >= Dt {
		s.acc -= Dt
		s.Tick()
		n++
	}
	return n
}

// Alpha is how far the next step is, 0…1: for interpolating the piece.
func (s *Sim) Alpha() float64 { return s.acc / Dt }

// Tick is one simulation step.
func (s *Sim) Tick() {
	s.Now += Dt
	s.Ticks++
	switch s.Phase {
	case Playing:
		s.pieceTick()
	case Attract:
		s.attract()
	}
	s.SandTick()
}

func (s *Sim) pieceTick() {
	p := &s.Cur
	b := s.B
	speed := FallSpeed(s.Level)
	if s.SoftDrop {
		speed = max(speed, SoftSpeed)
	}
	p.Fall += speed * float64(b) * Dt
	for p.Fall >= 1 {
		if !s.Fits(p.Kind, p.Rot, p.X, p.Y+1) {
			p.Fall = 0
			break
		}
		p.Fall--
		p.Y++
		s.resting = 0
		if s.SoftDrop && p.Y%b == 0 {
			s.Score++
			s.DropPts++
		}
	}
	if !s.Fits(p.Kind, p.Rot, p.X, p.Y+1) {
		s.resting += Dt
		if s.resting >= LockDelay || s.SoftDrop && s.resting >= LockDelay/4 {
			s.lock()
		}
	}
}

// attract plays by itself: a piece every so often, dropped at random, and a
// fresh board when it fills.
func (s *Sim) attract() {
	s.attractT -= Dt
	if s.attractT > 0 {
		return
	}
	s.attractT = 0.55
	p := s.draw()
	p.Rot = s.intn(4)
	s.Cur = p
	s.Cur.X = s.intn(s.GW - 2*s.B)
	s.Cur.Y = -4 * s.B
	for !s.Fits(p.Kind, p.Rot, s.Cur.X, s.Cur.Y) && s.Cur.X > 0 {
		s.Cur.X--
	}
	y := s.DropY()
	if y < 0 {
		s.SetSize(s.B)
		return
	}
	s.Cur.Y = y
	s.lock()
}

// SandTick lets the sand fall, finds a clear, and ends one.
func (s *Sim) SandTick() {
	if s.FlashT > 0 {
		s.FlashT -= Dt
		if s.FlashT <= 0 {
			s.FlashT = 0
			for _, i := range s.clr[:s.nclr] {
				s.Sand[i] = 0
				y := int(i) / s.GW
				s.wake(y-1, y-1)
			}
			s.nclr = 0
			s.emit(Event{Kind: EvVanish})
		}
	}
	if s.Settled {
		return
	}
	loose := s.fall()
	if s.GrainsMoved >= s.B*s.B {
		s.emit(Event{Kind: EvTrickle, N: s.GrainsMoved})
	}
	if s.FlashT == 0 {
		s.findClear()
	}
	if !loose && s.FlashT == 0 {
		s.Settled = true
	}
}

// fall moves the sand one step (see apps/lemondrop/game.go for the long
// explanation): a loose grain speeds up and moves a cell when it has gathered
// one, straight down if it can, else diagonally at random; sliding is slower
// and hesitates; the sweep is bottom-up and alternates direction; only awake
// rows are visited.
func (s *Sim) fall() bool {
	gw, gh := s.GW, s.GH
	sd := s.Sand[:gw*gh]
	vel, frac := s.vel[:gw*gh], s.frac[:gw*gh]
	top := uint8(min(31, 32*s.B/5))
	loose, left := false, false
	s.GrainsMoved = 0
	r := uint64(s.rng)
	for y := gh - 2; y >= 0; y-- {
		if !s.awake[y] && !left {
			continue
		}
		s.awake[y], left = false, false
		base := y * gw
		rev := (uint64(y)+s.Ticks)&1 == 0
		for k := 0; k < gw; k++ {
			x := k
			if rev {
				x = gw - 1 - k
			}
			i := base + x
			v := sd[i]
			if v == 0 || v&ClearBit != 0 {
				continue
			}
			down, to := i+gw, -1
			if sd[down] == 0 {
				to = down
			} else {
				r ^= r << 13
				r ^= r >> 7
				r ^= r << 17
				d := 1
				if r&1 == 0 {
					d = -1
				}
				switch {
				case x+d >= 0 && x+d < gw && sd[down+d] == 0:
					to = down + d
				case x-d >= 0 && x-d < gw && sd[down-d] == 0:
					to = down - d
				}
			}
			if to < 0 {
				if y+1 < gh-1 && vel[down] != 0 && sd[down]&ClearBit == 0 {
					frac[i] = 31
					s.awake[y] = true
					loose = true
				} else {
					vel[i], frac[i] = 0, 0
				}
				continue
			}
			sp := min(vel[i]+gravity, top)
			if to != down {
				sp = min(sp, top/2)
				if r&0x30 == 0 {
					vel[i] = max(sp, 1)
					s.awake[y] = true
					loose = true
					continue
				}
			}
			f := frac[i] + sp
			if f < 32 {
				vel[i], frac[i] = sp, f
				s.awake[y] = true
				loose = true
				continue
			}
			sd[i], vel[i], frac[i] = 0, 0, 0
			left = true
			s.GrainsMoved++
			if y+1 == gh-1 {
				sd[to] = v
				continue
			}
			sd[to], vel[to], frac[to] = v, sp, f-32
			loose = true
			s.awake[y+1] = true
		}
	}
	s.rng = RNG(r | 1)
	return loose
}

// findClear looks for a run of one colour from wall to wall (8-connected),
// starting only from grains on the left wall. A run found starts to flash.
func (s *Sim) findClear() {
	gw, gh := s.GW, s.GH
	s.stamp++
	if s.stamp == 0 {
		clear(s.mark[:])
		s.stamp = 1
	}
	st := s.stamp
	found := 0
	var colourOf uint8
	for y := 0; y < gh; y++ {
		i := y * gw
		v := s.Sand[i]
		if v == 0 || v&ClearBit != 0 || s.mark[i] == st {
			continue
		}
		c := v & ColourMask
		start := s.nclr
		s.mark[i] = st
		s.stack[0] = int32(i)
		sp := 1
		right := false
		for sp > 0 {
			sp--
			j := int(s.stack[sp])
			s.clr[s.nclr] = int32(j)
			s.nclr++
			x, yy := j%gw, j/gw
			if x == gw-1 {
				right = true
			}
			for ny := max(yy-1, 0); ny <= min(yy+1, gh-1); ny++ {
				for nx := max(x-1, 0); nx <= min(x+1, gw-1); nx++ {
					n := ny*gw + nx
					w := s.Sand[n]
					if w&(ColourMask|ClearBit) != c || s.mark[n] == st {
						continue
					}
					s.mark[n] = st
					s.stack[sp] = int32(n)
					sp++
				}
			}
		}
		if !right {
			s.nclr = start
			continue
		}
		for _, j := range s.clr[start:s.nclr] {
			s.Sand[j] |= ClearBit
		}
		found += s.nclr - start
		colourOf = c
	}
	if found == 0 {
		return
	}
	s.FlashT = FlashTime
	if s.Phase != Playing {
		s.emit(Event{Kind: EvClear, N: found, Colour: colourOf})
		return
	}
	s.Combo++
	s.Clears++
	level := s.Level
	s.Level = 1 + s.Clears/4
	base := (found*10 + s.B*s.B/2) / (s.B * s.B)
	if s.NLog < MaxLog {
		s.ClearLog[s.NLog] = [2]int32{int32(base), int32(s.Combo)}
	}
	s.NLog++
	pts := base * s.Level * s.Combo
	s.Score += pts
	s.emit(Event{Kind: EvClear, N: found, Base: base, Chain: s.Combo, Points: pts, Colour: colourOf})
	if s.Level > level {
		s.emit(Event{Kind: EvLevelUp, N: s.Level})
	}
}

// Clearing is the grains flashing now (board indices); valid until the next
// step. It allocates nothing: it is a view of the Sim's own array.
func (s *Sim) Clearing() []int32 { return s.clr[:s.nclr] }

// Grain is the byte at (x, y).
func (s *Sim) Grain(x, y int) uint8 { return s.Sand[y*s.GW+x] }

// StackTop is the highest occupied grain row (GH when the board is empty).
func (s *Sim) StackTop() int {
	for i, v := range s.Sand[:s.GW*s.GH] {
		if v != 0 {
			return i / s.GW
		}
	}
	return s.GH
}
