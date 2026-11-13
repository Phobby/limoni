// Package blocks is "Zest Blocks", the drag-and-place mode: an 8×8 board, a
// tray of three pieces, rows and columns that clear when full. It is a
// separate mode with its own rules and its own (local) leaderboard; it shares
// only the palette and the sand-burst look with the falling game.
//
// Rules (shown on the How to play screen):
//
//	placing a piece          +1 a cell
//	clearing L lines at once  80 × L × (L + Z) × combo, Z = zest lines
//	a line all of one colour  a "zest line": it counts twice in the multiplier
//	emptying the board        +300
//	combo                     +1 for every placement that clears; three
//	                          placements in a row without a clear end it
//	game over                 when no piece left in the tray fits anywhere
package blocks

import (
	"math/bits"

	"github.com/thebanri/limoni/apps/lemondrop-mobile/core"
)

const (
	N        = 8
	Colours  = core.Colours
	LinePts  = 80
	CleanPts = 300
	MissMax  = 3 // placements without a clear that end a combo
)

// Shape is a piece in its box, cells as bits r*N+c from the top-left.
type Shape struct {
	ID    int
	Mask  uint64
	W, H  int
	Cells int
}

// At is the shape's mask placed with its top-left at (r, c).
func (s Shape) At(r, c int) uint64 { return s.Mask << (r*N + c) }

// Has reports whether the shape's box cell (r, c) is filled.
func (s Shape) Has(r, c int) bool { return s.Mask>>(r*N+c)&1 != 0 }

// Shapes is the catalogue: generic polyominoes, each orientation its own shape.
var Shapes []Shape

var patterns = []string{
	"#",
	"##", "#/#",
	"###", "#/#/#",
	"####", "#/#/#/#",
	"#####", "#/#/#/#/#",
	"##/##",
	"###/###/###",
	"##/##/##", "###/###",
	"##/#.", "##/.#", "#./##", ".#/##",
	"#../###", "..#/###", "###/#..", "###/..#", "##/#./#.", "##/.#/.#", "#./#./##", ".#/.#/##",
	"###/.#.", ".#./###", "#./##/#.", ".#/##/.#",
	"##./.##", ".##/##.", "#./##/.#", ".#/##/#.",
	"###/#../#..", "###/..#/..#", "#../#../###", "..#/..#/###",
}

func init() {
	for id, p := range patterns {
		var sh Shape
		r, c := 0, 0
		for _, ch := range p {
			switch ch {
			case '/':
				r, c = r+1, 0
				continue
			case '#':
				sh.Mask |= 1 << (r*N + c)
				sh.Cells++
			}
			c++
			sh.W = max(sh.W, c)
		}
		sh.ID, sh.H = id, r+1
		Shapes = append(Shapes, sh)
	}
}

var rowMask, colMask [N]uint64

func init() {
	for i := 0; i < N; i++ {
		rowMask[i] = 0xff << (i * N)
		colMask[i] = 0x0101010101010101 << i
	}
}

// Board is the grid: occupancy bits and a colour per cell.
type Board struct {
	Occ    uint64
	Colour [N * N]uint8
}

// Fits reports whether shape s fits with its top-left at (r, c).
func Fits(occ uint64, s Shape, r, c int) bool {
	return r >= 0 && c >= 0 && r+s.H <= N && c+s.W <= N && occ&s.At(r, c) == 0
}

// AnyFit reports whether s fits anywhere.
func AnyFit(occ uint64, s Shape) bool {
	for r := 0; r+s.H <= N; r++ {
		for c := 0; c+s.W <= N; c++ {
			if occ&s.At(r, c) == 0 {
				return true
			}
		}
	}
	return false
}

// Full returns the full rows and columns of occ as bit sets (bit i: line i).
func Full(occ uint64) (rows, cols uint8) {
	for i := 0; i < N; i++ {
		if occ&rowMask[i] == rowMask[i] {
			rows |= 1 << i
		}
		if occ&colMask[i] == colMask[i] {
			cols |= 1 << i
		}
	}
	return
}

// ClearMask is the cells of the given rows and columns.
func ClearMask(rows, cols uint8) (m uint64) {
	for i := 0; i < N; i++ {
		if rows>>i&1 != 0 {
			m |= rowMask[i]
		}
		if cols>>i&1 != 0 {
			m |= colMask[i]
		}
	}
	return m
}

// PlaceOcc is placing s at (r, c) on occ and clearing: the new occupancy and
// how many lines went. For search; Game.Place does the full rules.
func PlaceOcc(occ uint64, s Shape, r, c int) (uint64, int) {
	occ |= s.At(r, c)
	rows, cols := Full(occ)
	return occ &^ ClearMask(rows, cols), bits.OnesCount8(rows) + bits.OnesCount8(cols)
}

// Slot is a piece in the tray.
type Slot struct {
	Shape  int // index into Shapes
	Colour uint8
	Used   bool
}

// Dealer fills the tray. It may read the game and must take its randomness
// only from r, so runs stay seeded.
type Dealer interface {
	Deal(g *Game, r *core.RNG) [3]Slot
}

// EventKind is feedback for the frontend; events never change the rules.
type EventKind uint8

const (
	EvPlaced  EventKind = iota + 1
	EvCleared           // Lines, Rows/Cols, Zest, Points
	EvComboLost
	EvClean
	EvDeal
	EvGameOver
)

type Event struct {
	Kind        EventKind
	Lines, Zest int
	Rows, Cols  uint8
	Points      int
	Combo       int
	Mask        uint64 // placed cells (EvPlaced) or cleared cells (EvCleared)
	Colours     [N * N]uint8
}

// Game is a run of Zest Blocks.
type Game struct {
	Board
	Tray   [3]Slot
	Dealer Dealer

	Score, Combo, Miss  int
	Lines, Placed, Best int
	Over                bool

	rng core.RNG
	ev  [16]Event
	nev int
}

// New starts a run.
func New(seed uint64, d Dealer) *Game {
	g := &Game{Dealer: d, rng: core.RNG(seed | 1)}
	g.deal()
	return g
}

func (g *Game) emit(e Event) {
	if g.nev < len(g.ev) {
		g.ev[g.nev] = e
		g.nev++
	}
}

// Events returns and forgets what happened since the last call.
func (g *Game) Events() []Event {
	e := g.ev[:g.nev]
	g.nev = 0
	return e
}

func (g *Game) deal() {
	if g.Dealer != nil {
		g.Tray = g.Dealer.Deal(g, &g.rng)
	} else {
		for i := range g.Tray {
			g.Tray[i] = Slot{Shape: g.rng.Intn(len(Shapes)), Colour: uint8(1 + g.rng.Intn(Colours))}
		}
	}
	g.emit(Event{Kind: EvDeal})
}

// CanPlace reports whether tray slot i fits at (r, c).
func (g *Game) CanPlace(i, r, c int) bool {
	s := g.Tray[i]
	return !g.Over && !s.Used && Fits(g.Occ, Shapes[s.Shape], r, c)
}

// Place puts tray slot i at (r, c): it scores, clears, deals a new tray when
// the old one is used up, and ends the run when nothing left fits.
func (g *Game) Place(i, r, c int) bool {
	if !g.CanPlace(i, r, c) {
		return false
	}
	slot := &g.Tray[i]
	sh := Shapes[slot.Shape]
	m := sh.At(r, c)
	g.Occ |= m
	for b := m; b != 0; b &= b - 1 {
		g.Colour[bits.TrailingZeros64(b)] = slot.Colour
	}
	slot.Used = true
	g.Placed++
	g.Score += sh.Cells
	g.emit(Event{Kind: EvPlaced, Mask: m, Points: sh.Cells})

	rows, cols := Full(g.Occ)
	lines := bits.OnesCount8(rows) + bits.OnesCount8(cols)
	if lines > 0 {
		g.Combo++
		g.Miss = 0
		zest := 0
		for k := 0; k < N; k++ {
			if rows>>k&1 != 0 && g.oneColour(rowMask[k]) {
				zest++
			}
			if cols>>k&1 != 0 && g.oneColour(colMask[k]) {
				zest++
			}
		}
		// Each line is worth 80 × L; a zest line counts twice.
		pts := LinePts * lines * (lines + zest) * g.Combo
		g.Score += pts
		g.Lines += lines
		cm := ClearMask(rows, cols)
		ev := Event{Kind: EvCleared, Lines: lines, Zest: zest, Rows: rows, Cols: cols, Points: pts, Combo: g.Combo, Mask: cm, Colours: g.Colour}
		g.Occ &^= cm
		for b := cm; b != 0; b &= b - 1 {
			g.Colour[bits.TrailingZeros64(b)] = 0
		}
		g.emit(ev)
		if g.Occ == 0 {
			g.Score += CleanPts
			g.emit(Event{Kind: EvClean, Points: CleanPts})
		}
	} else if g.Combo > 0 {
		g.Miss++
		if g.Miss >= MissMax {
			g.emit(Event{Kind: EvComboLost, Combo: g.Combo})
			g.Combo, g.Miss = 0, 0
		}
	}
	if g.Tray[0].Used && g.Tray[1].Used && g.Tray[2].Used {
		g.deal()
	}
	if !g.anyMove() {
		g.Over = true
		g.Best = max(g.Best, g.Score)
		g.emit(Event{Kind: EvGameOver})
	}
	return true
}

func (g *Game) oneColour(line uint64) bool {
	c := uint8(0)
	for b := line; b != 0; b &= b - 1 {
		v := g.Colour[bits.TrailingZeros64(b)]
		if c == 0 {
			c = v
		} else if v != c {
			return false
		}
	}
	return true
}

func (g *Game) anyMove() bool {
	for _, s := range g.Tray {
		if !s.Used && AnyFit(g.Occ, Shapes[s.Shape]) {
			return true
		}
	}
	return false
}

// Fill is the board's occupancy, 0…1.
func (g *Game) Fill() float64 { return float64(bits.OnesCount64(g.Occ)) / (N * N) }
