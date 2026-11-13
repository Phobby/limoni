package app

import (
	"os"
	"path/filepath"
)

// Haptic is a kind of touch feedback; the native shells map it to the
// platform's generators (UIImpactFeedbackGenerator / VibrationEffect).
type Haptic int

const (
	HapticLight   Haptic = iota // a valid action: turn, place
	HapticMedium                // a landing, a clear
	HapticHeavy                 // a hard drop
	HapticSuccess               // a big chain, a new best
	HapticError                 // game over
)

// Platform is what the game needs from the device. The desktop runner and
// the mobile bindings each provide one.
type Platform interface {
	// Haptic plays touch feedback; a no-op where there is none.
	Haptic(h Haptic)
	// RequestText asks the user for a line of text (the player's name). It
	// returns false when the platform has no native prompt; the game then
	// shows its own keyboard entry. The answer comes back via App.TextEntered.
	RequestText(prompt, current string) bool
	// SafeArea is the insets, in dp, that the game must not draw controls
	// under (notches, home indicator).
	SafeArea() (top, left, bottom, right float64)
	// DataDir is a writable directory for the save file.
	DataDir() string
}

// Desktop is the Platform for the desktop runner.
type Desktop struct {
	Insets  [4]float64 // simulated safe area, for testing phone layouts
	Haptics int        // count of haptic requests (tests, debug overlay)
	Dir     string
}

func (d *Desktop) Haptic(Haptic)                           { d.Haptics++ }
func (d *Desktop) RequestText(prompt, current string) bool { return false }
func (d *Desktop) SafeArea() (float64, float64, float64, float64) {
	return d.Insets[0], d.Insets[1], d.Insets[2], d.Insets[3]
}

func (d *Desktop) DataDir() string {
	if d.Dir != "" {
		return d.Dir
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "limoni")
}
