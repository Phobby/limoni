package app

import "github.com/hajimehoshi/ebiten/v2"

// The sand: one texel a grain (colour in rgb, alpha 1, flash in the texel's
// alpha being < 1 is not used; flashing grains are written pre-lit). Drawn
// scaled up, each fragment knows where it is inside its grain (fract of the
// source position) and shades a soft, lit, slightly irregular round grain.
const sandShaderSrc = `//kage:unit pixels
package main

var Time float
var Glow float

func hash(p vec2) float {
	return fract(sin(dot(p, vec2(12.9898, 78.233))) * 43758.5453)
}

func Fragment(dst vec4, src vec2, color vec4) vec4 {
	c := imageSrc0At(src)
	if c.a < 0.01 {
		return vec4(0)
	}
	cell := floor(src)
	f := fract(src) - 0.5
	j := hash(cell)
	// a grain is a rounded square, a little different each
	q := abs(f) - vec2(0.34+0.06*j)
	d := length(max(q, 0.0)) + min(max(q.x, q.y), 0.0)
	body := 1.0 - smoothstep(0.06, 0.16, d)
	light := 1.0 + 0.30*(-f.y) + 0.12*(-f.x) - 0.10*j
	spec := smoothstep(0.22, 0.0, length(f-vec2(-0.16, -0.18))) * 0.22
	col := c.rgb*light + vec3(spec)
	// a faint fill between grains so the pile reads as a mass, not a grid
	base := c.rgb * 0.62
	col = mix(base, col, body)
	return vec4(col*Glow, 1.0) * color.a
}
`

// The background: a deep gradient, a slowly turning lemon slice behind the
// board (Lemon Drop's own lemon: rind, pith, ten segments), soft bokeh.
const bgShaderSrc = `//kage:unit pixels
package main

var Time float
var Size vec2
var Lemon vec3 // centre x, y, radius in pixels
var Amount float
var Top vec3
var Bottom vec3

func Fragment(dst vec4, src vec2, color vec4) vec4 {
	uv := dst.xy / Size
	col := mix(Top, Bottom, smoothstep(0.0, 1.0, uv.y))
	// soft bokeh: a few drifting discs
	for i := 0; i < 6; i++ {
		fi := float(i)
		p := vec2(fract(0.17*fi + 0.031*Time*(0.4+0.1*fi)), fract(0.61*fi + 0.013*Time))
		p = vec2(p.x, 1.0-p.y) * Size
		r := (0.07 + 0.03*fi) * Size.x
		d := length(dst.xy - p)
		col += vec3(1.0, 0.85, 0.35) * 0.035 * (1.0 - smoothstep(r*0.6, r, d))
	}
	// the lemon
	v := (dst.xy - Lemon.xy) / Lemon.z
	d := length(v)
	if d < 1.0 {
		lc := vec3(1.0, 0.85, 0.29) * (0.86 + 0.14*d)
		if d > 0.9 {
			lc = vec3(0.95, 0.76, 0.0)
		} else if d > 0.8 || d < 0.1 {
			lc = vec3(1.0, 0.95, 0.77)
		} else {
			a := (atan2(v.y, v.x) + 3.14159265) / 6.2831853 + Time/90.0
			seg := fract(a * 10.0)
			arc := min(seg, 1.0-seg) * 6.2831853 / 10.0 * d
			if arc < 0.035 {
				lc = vec3(1.0, 0.95, 0.77)
			}
		}
		edge := 1.0 - smoothstep(0.985, 1.0, d)
		col = mix(col, lc, Amount*edge)
	}
	// vignette
	vg := length(uv - vec2(0.5, 0.45))
	col *= 1.0 - 0.35*smoothstep(0.35, 0.9, vg)
	return vec4(col, 1.0)
}
`

// The board's frame: a dark panel with an inner glow and a citrus edge.
const panelShaderSrc = `//kage:unit pixels
package main

var Rect vec4 // x, y, w, h in pixels
var Radius float
var Pulse float
var Edge vec3

func sdRound(p vec2, b vec2, r float) float {
	q := abs(p) - b + r
	return length(max(q, 0.0)) + min(max(q.x, q.y), 0.0) - r
}

func Fragment(dst vec4, src vec2, color vec4) vec4 {
	c := Rect.xy + Rect.zw*0.5
	d := sdRound(dst.xy-c, Rect.zw*0.5, Radius)
	inside := 1.0 - smoothstep(-0.5, 0.5, d)
	glow := exp(-max(d, 0.0) / (6.0 + 6.0*Pulse)) * (0.35 + 0.4*Pulse)
	panel := mix(vec3(0.055, 0.06, 0.09), vec3(0.085, 0.09, 0.13), smoothstep(Rect.y+Rect.w, Rect.y, dst.y))
	inner := smoothstep(-24.0, 0.0, d) * 0.10
	col := panel + Edge*inner
	edge := 1.0 - smoothstep(0.0, 2.2, abs(d+1.2))
	col = mix(col, Edge, edge*0.85)
	a := max(inside, glow*(1.0-inside))
	out := mix(Edge*glow, col, inside)
	return vec4(out*a, a) * color.a
}
`

var sandShader, bgShader, panelShader *ebiten.Shader

func loadShaders() {
	must := func(src string) *ebiten.Shader {
		s, err := ebiten.NewShader([]byte(src))
		if err != nil {
			panic(err)
		}
		return s
	}
	sandShader = must(sandShaderSrc)
	bgShader = must(bgShaderSrc)
	panelShader = must(panelShaderSrc)
}
