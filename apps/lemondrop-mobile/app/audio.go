package app

import (
	"encoding/binary"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hajimehoshi/ebiten/v2/audio"
)

// Sound is Lemon Drop's: every effect synthesised at start-up from tones and
// filtered noise (ported from apps/lemondrop/sound.go), mixed here into one
// float32 stereo stream that Ebitengine plays on desktop, iOS and Android.
// The game side never blocks: play drops a request into a buffered channel.

type sfx uint8

const (
	sfxMove sfx = iota
	sfxTurn
	sfxLand
	sfxDrop
	sfxTrickle
	sfxClear
	sfxChain
	sfxLevel
	sfxStart
	sfxPause
	sfxOver
	sfxPlace
	sfxTap
	sfxHigh
	nSfx
)

const sampleRate = 44100

type sfxReq struct {
	s        sfx
	vol, pan float32
}

type voice struct {
	clip   []float32
	pos    int
	gl, gr float32
}

// mixer is an endless io.Reader of float32 LE stereo.
type mixer struct {
	req    chan sfxReq
	clips  [nSfx][]float32
	music  []float32 // a stereo-interleaved loop
	musPos int
	sfxVol atomic.Uint32
	musVol atomic.Uint32
	voices [32]voice
	mu     sync.Mutex
}

func newMixer() *mixer {
	m := &mixer{req: make(chan sfxReq, 64)}
	m.setVolumes(1, 0.5)
	m.synth()
	m.music = makeMusic()
	return m
}

func (m *mixer) setVolumes(sfxV, musV float32) {
	m.sfxVol.Store(math.Float32bits(sfxV))
	m.musVol.Store(math.Float32bits(musV))
}

func (m *mixer) play(s sfx, vol, pan float32) {
	select {
	case m.req <- sfxReq{s, vol, pan}:
	default:
	}
}

func (m *mixer) Read(p []byte) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for {
		select {
		case r := <-m.req:
			for i := range m.voices {
				if m.voices[i].clip == nil {
					l := r.vol * min(1, 1-r.pan)
					rr := r.vol * min(1, 1+r.pan)
					m.voices[i] = voice{clip: m.clips[r.s], gl: l, gr: rr}
					break
				}
			}
			continue
		default:
		}
		break
	}
	sv := math.Float32frombits(m.sfxVol.Load())
	mv := math.Float32frombits(m.musVol.Load())
	frames := len(p) / 8
	for i := 0; i < frames; i++ {
		var l, r float32
		for v := range m.voices {
			vc := &m.voices[v]
			if vc.clip == nil {
				continue
			}
			s := vc.clip[vc.pos]
			l += s * vc.gl
			r += s * vc.gr
			if vc.pos++; vc.pos >= len(vc.clip) {
				vc.clip = nil
			}
		}
		l, r = l*sv, r*sv
		if mv > 0 && len(m.music) > 0 {
			l += m.music[m.musPos] * mv
			r += m.music[m.musPos+1] * mv
		}
		if m.musPos += 2; m.musPos >= len(m.music) {
			m.musPos = 0
		}
		binary.LittleEndian.PutUint32(p[i*8:], math.Float32bits(softClip(l)))
		binary.LittleEndian.PutUint32(p[i*8+4:], math.Float32bits(softClip(r)))
	}
	return frames * 8, nil
}

func softClip(v float32) float32 { return float32(math.Tanh(float64(v))) }

// sound is the game's audio output: a mixer and the Ebitengine player.
type sound struct {
	m      *mixer
	player *audio.Player
}

var audioCtx *audio.Context

func newSound() *sound {
	if audioCtx == nil {
		audioCtx = audio.NewContext(sampleRate)
	}
	m := newMixer()
	p, err := audioCtx.NewPlayerF32(m)
	if err != nil {
		return &sound{m: m}
	}
	p.SetBufferSize(60 * time.Millisecond)
	p.Play()
	return &sound{m: m, player: p}
}

func (s *sound) play(e sfx, vol, pan float32) {
	if s != nil && s.player != nil {
		s.m.play(e, vol, pan)
	}
}

// ── synthesis (ported) ───────────────────────────────────────────────────

type synth struct {
	buf  []float32
	seed uint32
}

func newSynth(seconds float64) *synth {
	return &synth{buf: make([]float32, int(seconds*sampleRate)), seed: 12345}
}

func (s *synth) noise() float32 {
	s.seed ^= s.seed << 13
	s.seed ^= s.seed >> 17
	s.seed ^= s.seed << 5
	return float32(s.seed)/float32(math.MaxUint32)*2 - 1
}

func (s *synth) tone(t0, t1, f0, f1, amp, decay float64, wave func(float64) float64) {
	i0, i1 := int(t0*sampleRate), int(t1*sampleRate)
	phase := 0.0
	for i := i0; i < i1 && i < len(s.buf); i++ {
		t := float64(i-i0) / float64(i1-i0)
		f := f0 * math.Pow(f1/f0, t)
		phase += f / sampleRate
		env := math.Min(1, t*(t1-t0)*400) * math.Exp(-t*decay)
		s.buf[i] += float32(wave(phase) * amp * env)
	}
}

func (s *synth) hiss(t0, t1, cutoff, cutoff1, amp, decay float64) {
	i0, i1 := int(t0*sampleRate), int(t1*sampleRate)
	var y float32
	for i := i0; i < i1 && i < len(s.buf); i++ {
		t := float64(i-i0) / float64(i1-i0)
		c := cutoff * math.Pow(cutoff1/cutoff, t)
		a := float32(1 - math.Exp(-2*math.Pi*c/sampleRate))
		y += (s.noise() - y) * a
		env := math.Min(1, t*(t1-t0)*300) * math.Exp(-t*decay)
		s.buf[i] += y * float32(amp*env)
	}
}

func sine(p float64) float64   { return math.Sin(2 * math.Pi * p) }
func square(p float64) float64 { return math.Copysign(0.6, math.Sin(2*math.Pi*p)) }
func tri(p float64) float64    { return 4*math.Abs(p-math.Floor(p+0.5)) - 1 }

func (m *mixer) synth() {
	mk := func(sec float64, f func(s *synth)) []float32 {
		s := newSynth(sec)
		f(s)
		return s.buf
	}
	m.clips[sfxMove] = mk(0.04, func(s *synth) { s.tone(0, 0.04, 900, 800, 0.10, 8, tri) })
	m.clips[sfxTurn] = mk(0.07, func(s *synth) {
		s.tone(0, 0.07, 700, 1100, 0.14, 5, tri)
		s.hiss(0, 0.03, 4000, 2000, 0.06, 8)
	})
	m.clips[sfxLand] = mk(0.35, func(s *synth) {
		s.tone(0, 0.12, 150, 70, 0.45, 5, sine)
		s.hiss(0, 0.35, 2500, 600, 0.35, 3.5)
	})
	m.clips[sfxDrop] = mk(0.4, func(s *synth) {
		s.hiss(0, 0.1, 800, 3500, 0.25, 1)
		s.tone(0.08, 0.22, 120, 55, 0.6, 4, sine)
		s.hiss(0.08, 0.4, 2800, 500, 0.4, 3)
	})
	m.clips[sfxTrickle] = mk(0.14, func(s *synth) {
		s.hiss(0, 0.14, 5000, 3000, 0.18, 1.5)
		for t := 0.0; t < 0.13; t += 0.023 {
			s.hiss(t, t+0.012, 7000, 5000, 0.10, 6)
		}
	})
	m.clips[sfxClear] = mk(0.7, func(s *synth) {
		for i, f := range [3]float64{1047, 1319, 1568} {
			t := float64(i) * 0.05
			s.tone(t, t+0.5, f, f, 0.2, 4, sine)
			s.tone(t, t+0.5, f*2, f*2, 0.05, 6, sine)
		}
		s.hiss(0, 0.7, 6000, 900, 0.3, 3)
	})
	m.clips[sfxChain] = mk(0.8, func(s *synth) {
		for i, f := range [4]float64{1319, 1568, 1976, 2637} {
			t := float64(i) * 0.06
			s.tone(t, t+0.5, f, f, 0.18, 4, sine)
			s.tone(t, t+0.5, f/2, f/2, 0.1, 4, tri)
		}
		s.hiss(0, 0.8, 7000, 1200, 0.3, 3)
	})
	m.clips[sfxLevel] = mk(0.9, func(s *synth) {
		for i, f := range [...]float64{523, 659, 784, 1047} {
			t, d := float64(i)*0.09, 0.25
			if i == 3 {
				d = 0.6
			}
			s.tone(t, t+d, f, f, 0.18, 3, square)
		}
	})
	m.clips[sfxStart] = mk(0.45, func(s *synth) {
		s.tone(0, 0.12, 784, 784, 0.18, 3, square)
		s.tone(0.12, 0.45, 1175, 1175, 0.18, 4, square)
	})
	m.clips[sfxPause] = mk(0.1, func(s *synth) { s.tone(0, 0.1, 1200, 800, 0.14, 5, tri) })
	m.clips[sfxOver] = mk(1.4, func(s *synth) {
		for i, f := range [...]float64{392, 330, 277, 220} {
			t := float64(i) * 0.25
			s.tone(t, t+0.5, f, f*0.97, 0.2, 3, square)
		}
		s.hiss(0.9, 1.4, 1500, 300, 0.3, 3)
	})
	m.clips[sfxPlace] = mk(0.18, func(s *synth) {
		s.tone(0, 0.08, 220, 140, 0.35, 5, sine)
		s.hiss(0, 0.18, 3000, 900, 0.18, 4)
	})
	m.clips[sfxTap] = mk(0.05, func(s *synth) { s.tone(0, 0.05, 1500, 1300, 0.10, 7, tri) })
	m.clips[sfxHigh] = mk(1.6, func(s *synth) {
		for i, f := range [...]float64{784, 988, 1175, 1568, 1976} {
			t := float64(i) * 0.11
			s.tone(t, t+0.9, f, f, 0.14, 2.5, sine)
			s.tone(t, t+0.9, f*1.5, f*1.5, 0.04, 3, sine)
		}
		s.hiss(0.4, 1.6, 9000, 3000, 0.12, 2)
	})
}

// makeMusic is a calm 8-bar loop: soft pads under a plucked pentatonic
// figure, generated once. It is quiet by default and has its own volume.
func makeMusic() []float32 {
	const bpm = 92.0
	beat := 60 / bpm
	bars := 8
	n := int(float64(bars*4) * beat * sampleRate)
	l := newSynth(float64(n) / sampleRate)
	r := newSynth(float64(n) / sampleRate)
	chords := [4][3]float64{{220, 277.2, 329.6}, {196, 246.9, 293.7}, {174.6, 220, 261.6}, {196, 246.9, 329.6}}
	pent := [...]float64{440, 493.9, 554.4, 659.3, 740, 880}
	seed := uint32(7)
	for bar := 0; bar < bars; bar++ {
		ch := chords[bar%4]
		t0 := float64(bar*4) * beat
		for i, f := range ch {
			l.tone(t0, t0+4*beat, f, f, 0.035, 0.8, sine)
			r.tone(t0+0.01*float64(i), t0+4*beat, f*1.002, f*1.002, 0.035, 0.8, sine)
		}
		for k := 0; k < 8; k++ {
			seed ^= seed << 13
			seed ^= seed >> 17
			seed ^= seed << 5
			if seed%3 == 0 {
				continue
			}
			f := pent[int(seed>>8)%len(pent)]
			t := t0 + float64(k)*beat/2
			pl := l
			if k%2 == 1 {
				pl = r
			}
			pl.tone(t, t+beat*0.9, f, f, 0.05, 5, tri)
		}
	}
	out := make([]float32, 2*n)
	for i := 0; i < n; i++ {
		// gentle fade at the loop edges to avoid clicks
		e := float32(min(1, float64(i)/2000, float64(n-1-i)/2000))
		out[2*i], out[2*i+1] = l.buf[i]*e, r.buf[i]*e
	}
	return out
}
