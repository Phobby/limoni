package blocks

import "testing"

func shapeID(t *testing.T, w, h, cells int) int {
	t.Helper()
	for i, s := range Shapes {
		if s.W == w && s.H == h && s.Cells == cells {
			return i
		}
	}
	t.Fatalf("no %dx%d shape with %d cells", w, h, cells)
	return 0
}

func TestCatalogueIsWellFormed(t *testing.T) {
	seen := map[uint64]bool{}
	for _, s := range Shapes {
		if s.W < 1 || s.H < 1 || s.W > 5 || s.H > 5 || s.Cells < 1 {
			t.Fatalf("bad shape %+v", s)
		}
		if seen[s.Mask] {
			t.Fatalf("duplicate shape %+v", s)
		}
		seen[s.Mask] = true
	}
}

func TestRowsAndColumnsClearTogether(t *testing.T) {
	g := &Game{}
	g.Tray = [3]Slot{{Shape: shapeID(t, 1, 1, 1), Colour: 1}, {Shape: 0, Colour: 2}, {Shape: 0, Colour: 3}}
	// Row 0 full except (0,0); column 0 full except (0,0).
	for c := 1; c < N; c++ {
		g.Occ |= 1 << c
		g.Colour[c] = 1
	}
	for r := 1; r < N; r++ {
		g.Occ |= 1 << (r * N)
		g.Colour[r*N] = 2
	}
	if !g.Place(0, 0, 0) {
		t.Fatal("could not place the single cell")
	}
	if g.Occ != 0 {
		t.Fatalf("board not cleared: %064b", g.Occ)
	}
	// 2 lines: 80 × 2 × (2 + zest) × combo 1; row 0 is all colour 1 → zest 1.
	want := 1 + LinePts*2*(2+1)*1 + CleanPts
	if g.Score != want {
		t.Fatalf("score %d, want %d", g.Score, want)
	}
}

func TestComboGrowsAndEndsAfterThreeMisses(t *testing.T) {
	g := &Game{}
	one := shapeID(t, 1, 1, 1)
	g.Tray = [3]Slot{{Shape: one, Colour: 1}, {Shape: one, Colour: 1}, {Shape: one, Colour: 1}}
	fillRowBut := func(r, c int) {
		for x := 0; x < N; x++ {
			if x != c {
				g.Occ |= 1 << (r*N + x)
				g.Colour[r*N+x] = 4
			}
		}
	}
	fillRowBut(7, 0)
	fillRowBut(6, 0)
	g.Place(0, 7, 0)
	g.Place(1, 6, 0)
	if g.Combo != 2 {
		t.Fatalf("combo %d after two clearing placements", g.Combo)
	}
	// Three placements without a clear end it.
	g.Tray = [3]Slot{{Shape: one, Colour: 2}, {Shape: one, Colour: 2}, {Shape: one, Colour: 2}}
	g.Place(0, 0, 0)
	g.Place(1, 0, 2)
	if g.Combo != 2 {
		t.Fatalf("combo dropped early: %d", g.Combo)
	}
	g.Place(2, 0, 4)
	if g.Combo != 0 {
		t.Fatalf("combo %d after three misses, want 0", g.Combo)
	}
}

func TestGameEndsWhenNothingFits(t *testing.T) {
	g := &Game{}
	big := shapeID(t, 3, 3, 9)
	g.Occ = ^uint64(0)
	for i := 0; i < N; i++ { // one free cell per row and column: no full line
		g.Occ &^= 1 << (i*N + i)
	}
	g.Tray = [3]Slot{{Shape: shapeID(t, 1, 1, 1), Colour: 1}, {Shape: big, Colour: 1}, {Shape: big, Colour: 1}}
	g.Place(0, 0, 0)
	if !g.Over {
		t.Fatal("the run should end: only 3×3s left and no 3×3 hole")
	}
}
