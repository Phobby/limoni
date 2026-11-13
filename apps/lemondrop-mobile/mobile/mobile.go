// Package mobile is the entry point for `ebitenmobile bind`: it registers the
// game with Ebitengine's mobile view and exposes a few calls the native
// shells (Swift on iOS, Kotlin on Android) make. Names here become
// Mobile.* in Objective-C/Swift and mobile.Mobile.* in Java/Kotlin.
package mobile

import (
	"os"
	"sync"

	"github.com/hajimehoshi/ebiten/v2"
	ebmobile "github.com/hajimehoshi/ebiten/v2/mobile"

	"github.com/thebanri/limoni/apps/lemondrop-mobile/app"
)

// Bridge is implemented natively: haptics and a text prompt.
type Bridge interface {
	// Haptic plays feedback: 0 light, 1 medium, 2 heavy, 3 success, 4 error.
	Haptic(kind int)
	// RequestText shows a native text prompt; the shell answers with
	// TextEntered. It returns false when it cannot show one.
	RequestText(prompt, current string) bool
}

type platform struct {
	mu     sync.Mutex
	bridge Bridge
	insets [4]float64
	dir    string
}

func (p *platform) Haptic(h app.Haptic) {
	p.mu.Lock()
	b := p.bridge
	p.mu.Unlock()
	if b != nil {
		b.Haptic(int(h))
	}
}

func (p *platform) RequestText(prompt, current string) bool {
	p.mu.Lock()
	b := p.bridge
	p.mu.Unlock()
	return b != nil && b.RequestText(prompt, current)
}

func (p *platform) SafeArea() (float64, float64, float64, float64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.insets[0], p.insets[1], p.insets[2], p.insets[3]
}

func (p *platform) DataDir() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.dir
}

var (
	plat = &platform{}
	game = &lazy{}
)

// lazy builds the App on the first frame, after the shell has had the chance
// to call SetDataDir and SetBridge.
type lazy struct {
	once sync.Once
	a    *app.App
}

func (l *lazy) get() *app.App {
	// LEMONDROP_START opens a screen directly (simulator smoke tests:
	// SIMCTL_CHILD_LEMONDROP_START=drop xcrun simctl launch …).
	l.once.Do(func() { l.a = app.New(plat, app.Options{StartAt: os.Getenv("LEMONDROP_START")}) })
	return l.a
}

func (l *lazy) Update() error              { return l.get().Update() }
func (l *lazy) Draw(dst *ebiten.Image)     { l.get().Draw(dst) }
func (l *lazy) Layout(w, h int) (int, int) { return w, h }
func (l *lazy) LayoutF(w, h float64) (float64, float64) {
	return l.get().LayoutF(w, h)
}

func init() {
	ebiten.SetTPS(ebiten.SyncWithFPS)
	ebmobile.SetGame(game)
}

// SetDataDir sets the writable directory for save data (call before the
// first frame): Application Support on iOS, Context.getFilesDir on Android.
func SetDataDir(dir string) {
	plat.mu.Lock()
	plat.dir = dir
	plat.mu.Unlock()
}

// SetSafeArea gives the insets in points/dp: top, left, bottom, right.
func SetSafeArea(top, left, bottom, right float64) {
	plat.mu.Lock()
	plat.insets = [4]float64{top, left, bottom, right}
	plat.mu.Unlock()
}

// SetBridge registers the native haptics and text prompt.
func SetBridge(b Bridge) {
	plat.mu.Lock()
	plat.bridge = b
	plat.mu.Unlock()
}

// TextEntered delivers the text prompt's answer.
func TextEntered(s string) { game.get().TextEntered(s) }

// Suspend tells the game the app is going to the background: it pauses a
// run and saves. (Ebitengine also suspends its own loop and audio.)
func Suspend() { game.get().Suspend() }
