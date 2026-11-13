// Command lemondrop-mobile runs Lemon Drop Mobile on the desktop, in a
// phone-shaped window: for development, play-testing and screenshots.
//
//	go run ./cmd/lemondrop-mobile
//	go run ./cmd/lemondrop-mobile -start drop -speed 4 -frames 600 -shot drop.png
package main

import (
	"flag"
	"image/png"
	"log"
	"os"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/thebanri/limoni/apps/lemondrop-mobile/app"
)

type runner struct {
	*app.App
	frames int
	shot   string
	n      int
}

func (r *runner) Update() error {
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
	start := flag.String("start", "", "start screen: menu, modes, drop, fair, zest, settings, help, board")
	speed := flag.Float64("speed", 0, "fixed time multiplier per frame (screenshots)")
	frames := flag.Int("frames", 0, "quit after this many frames")
	shot := flag.String("shot", "", "save a PNG of the last frame")
	seed := flag.Uint64("seed", 0, "random seed (0: clock)")
	mute := flag.Bool("mute", false, "no sound")
	board := flag.String("board", "", `world leaderboard URL; "off" disables`)
	data := flag.String("data", "", "save-data directory (default: user config dir)")
	flag.Parse()

	plat := &app.Desktop{Insets: [4]float64{*top, 0, *bottom, 0}, Dir: *data}
	g := app.New(plat, app.Options{BoardURL: *board, Seed: *seed, NoSound: *mute, StartAt: *start, Speed: *speed})
	ebiten.SetWindowSize(*w, *h)
	ebiten.SetWindowTitle("Lemon Drop Mobile")
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	ebiten.SetTPS(ebiten.SyncWithFPS)
	if err := ebiten.RunGame(&runner{App: g, frames: *frames, shot: *shot}); err != nil {
		log.Fatal(err)
	}
}
