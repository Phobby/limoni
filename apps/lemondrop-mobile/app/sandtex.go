package app

import (
	"math"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/thebanri/limoni/apps/lemondrop-mobile/core"
)

// sandTex is the board as one texel a grain: filled from the Sim's bytes
// through a 256-entry colour table, uploaded once a frame (6 480 texels) and
// drawn with the sand shader in a single draw call.
type sandTex struct {
	img *ebiten.Image
	pix []byte
	lut [256][4]byte
	gw  int
	uni map[string]any
	t   []float32
	g   []float32
}

var shadeK = [4]float32{0.72, 0.88, 1.0, 1.12}

func newSandTex() *sandTex {
	s := &sandTex{uni: map[string]any{}, t: []float32{0}, g: []float32{1}}
	s.uni["Time"], s.uni["Glow"] = s.t, s.g
	for v := 0; v < 256; v++ {
		c := v & core.ColourMask
		if c == 0 || c > core.Colours {
			continue
		}
		k := shadeK[v>>core.ShadeShift&3]
		col := fruit[c].scale(k)
		if k > 1 {
			col = fruit[c].lighten(k - 1)
		}
		s.lut[v] = [4]byte{byte(min(1, col.r) * 255), byte(min(1, col.g) * 255), byte(min(1, col.b) * 255), 255}
	}
	return s
}

func (s *sandTex) ensure(gw, gh int) {
	if s.img != nil && s.gw == gw && s.img.Bounds().Dy() == gh {
		return
	}
	s.img = ebiten.NewImage(gw, gh)
	s.pix = make([]byte, gw*gh*4)
	s.gw = gw
}

// fill writes the Sim's sand into the texture; flashing grains pulse to white.
func (s *sandTex) fill(sim *core.Sim, t float64) {
	s.ensure(sim.GW, sim.GH)
	pulse := float32(0.5 + 0.5*math.Sin(t*28))
	flash := [4]byte{byte(255), byte(230 + 25*pulse), byte(140 + 100*pulse), 255}
	sand := sim.Sand[:sim.GW*sim.GH]
	p := s.pix
	for i, v := range sand {
		var c [4]byte
		if v&core.ClearBit != 0 {
			c = flash
		} else {
			c = s.lut[v]
		}
		j := i * 4
		p[j], p[j+1], p[j+2], p[j+3] = c[0], c[1], c[2], c[3]
	}
	s.img.WritePixels(p)
}

// draw puts the sand at (x, y) dp with grain size g dp.
func (s *sandTex) draw(c *canvas, x, y, g float64, t float64, glow float32) {
	s.t[0], s.g[0] = float32(t), glow
	op := &ebiten.DrawRectShaderOptions{Uniforms: s.uni}
	op.Images[0] = s.img
	op.GeoM.Scale(g*float64(c.s), g*float64(c.s))
	op.GeoM.Translate(float64(c.px(x)), float64(c.px(y)))
	b := s.img.Bounds()
	c.dst.DrawRectShader(b.Dx(), b.Dy(), sandShader, op)
}

// drawBlock is one block of a falling piece: a rounded, lit tile.
func drawBlock(c *canvas, x, y, size float64, col rgb, a float32) {
	c.blit(c.sprite(sprBlock, col, size), x, y, a)
}

// drawPiece draws a piece's four blocks with its box top-left at (x, y) dp.
func drawPiece(c *canvas, kind, rot int, col rgb, x, y, block float64, a float32) {
	for _, cell := range core.Shapes[kind][rot] {
		drawBlock(c, x+float64(cell[0])*block, y+float64(cell[1])*block, block, col, a)
	}
}

// drawGhost outlines where a piece would land.
func drawGhost(c *canvas, kind, rot int, col rgb, x, y, block float64) {
	for _, cell := range core.Shapes[kind][rot] {
		c.blit(c.sprite(sprGhost, col, block), x+float64(cell[0])*block, y+float64(cell[1])*block, 1)
	}
}
