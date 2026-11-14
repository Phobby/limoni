// Command lemondrop-mobile runs Lemon Drop Mobile on the desktop, in a
// phone-shaped window: for development, play-testing and screenshots.
//
//	go run ./cmd/lemondrop-mobile
//	go run ./cmd/lemondrop-mobile -start drop -speed 4 -frames 600 -shot drop.png
package main

import (
	"flag"
	"fmt"
	"image/png"
	"log"
	"os"
	"runtime"
	"runtime/pprof"
	"sort"
	"time"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/thebanri/limoni/apps/lemondrop-mobile/app"
)

type runner struct {
	*app.App
	frames int
	shot   string
	n      int

	bench    bool
	cpu      []time.Duration // Update+Draw time per frame
	t0       time.Time
	ms0, ms1 runtime.MemStats
}

func (r *runner) Update() error {
	if r.bench {
		r.t0 = time.Now()
		if r.n == 60 { // after warm-up
			runtime.ReadMemStats(&r.ms0)
		}
	}
	if err := r.App.Update(); err != nil {
		return err
	}
	if r.frames > 0 && r.n > r.frames+1 {
		return ebiten.Termination
	}
	return nil
}

func (r *runner) Draw(dst *ebiten.Image) {
	r.App.Draw(dst)
	if r.bench && r.n >= 60 {
		r.cpu = append(r.cpu, time.Since(r.t0))
		if r.n == r.frames-1 {
			runtime.ReadMemStats(&r.ms1)
			r.report()
		}
	}
	r.n++
	if r.frames > 0 && r.n == r.frames && r.shot != "" {
		f, err := os.Create(r.shot)
		if err != nil {
			log.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, dst); err != nil {
			log.Fatal(err)
		}
	}
}

func main() {
	w := flag.Int("w", 393, "window width in dp (iPhone 15/16: 393)")
	h := flag.Int("h", 852, "window height in dp")
	top := flag.Float64("inset-top", 0, "simulated safe-area top inset, dp (notch: 59)")
	bottom := flag.Float64("inset-bottom", 0, "simulated safe-area bottom inset, dp (home bar: 34)")
	start := flag.String("start", "", "start screen: menu, drop, settings, help, board")
	speed := flag.Float64("speed", 0, "fixed time multiplier per frame (screenshots)")
	frames := flag.Int("frames", 0, "quit after this many frames")
	shot := flag.String("shot", "", "save a PNG of the last frame")
	seed := flag.Uint64("seed", 0, "random seed (0: clock)")
	mute := flag.Bool("mute", false, "no sound")
	data := flag.String("data", "", "save-data directory (default: user config dir)")
	bench := flag.Bool("bench", false, "print per-frame CPU time and allocations (use with -frames)")
	memprof := flag.String("memprofile", "", "write an allocation profile at exit (with -bench)")
	flag.Parse()

	plat := &app.Desktop{Insets: [4]float64{*top, 0, *bottom, 0}, Dir: *data}
	g := app.New(plat, app.Options{Seed: *seed, NoSound: *mute, StartAt: *start, Speed: *speed})
	ebiten.SetWindowSize(*w, *h)
	ebiten.SetWindowTitle("Lemon Drop Mobile")
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	ebiten.SetTPS(ebiten.SyncWithFPS)
	if *memprof != "" {
		runtime.MemProfileRate = 512
	}
	if err := ebiten.RunGame(&runner{App: g, frames: *frames, shot: *shot, bench: *bench}); err != nil {
		log.Fatal(err)
	}
	if *memprof != "" {
		f, err := os.Create(*memprof)
		if err == nil {
			pprof.Lookup("allocs").WriteTo(f, 0)
			f.Close()
		}
	}
}

// report prints CPU time per frame (Update+Draw on the CPU; GPU work is
// asynchronous) and allocation/GC over the measured frames.
func (r *runner) report() {
	d := append([]time.Duration(nil), r.cpu...)
	sort.Slice(d, func(i, j int) bool { return d[i] < d[j] })
	var sum time.Duration
	for _, v := range d {
		sum += v
	}
	n := len(d)
	fmt.Printf("frames %d  cpu/frame mean %v  p50 %v  p99 %v  max %v\n", n, sum/time.Duration(n), d[n/2], d[n*99/100], d[n-1])
	fmt.Printf("alloc/frame %.0f B  mallocs/frame %.1f  GCs %d  TPS %.1f FPS %.1f\n",
		float64(r.ms1.TotalAlloc-r.ms0.TotalAlloc)/float64(n), float64(r.ms1.Mallocs-r.ms0.Mallocs)/float64(n),
		r.ms1.NumGC-r.ms0.NumGC, ebiten.ActualTPS(), ebiten.ActualFPS())
}
