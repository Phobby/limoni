# Device test checklist

Tick each item on at least one small phone, one large phone and one
high-refresh device per platform. Status today: iOS simulator smoke-tested
(menu and game render and run); no physical device yet.

## Install and lifecycle
- [ ] Cold start shows the splash, then the menu, in under 2 s
- [ ] Home / app switcher during a run, then back: the run is paused and nothing is lost
- [ ] Phone call / Control Center / notification shade during a run: paused, audio resumes after
- [ ] Kill the app during a run, relaunch: settings and boards are intact

## Layout
- [ ] Notch / Dynamic Island / punch-hole: score and pause are never covered
- [ ] Home indicator / gesture bar: the bottom controls are reachable and do not trigger system gestures
- [ ] Smallest supported phone (iPhone SE, 360×640 Android): whole board visible, settings scroll
- [ ] Tablet portrait: board centred, not stretched

## Falling mode (gestures)
- [ ] Drag left and right: the piece follows the finger in half-block steps, no lag
- [ ] Tap right half turns clockwise, left half turns back
- [ ] Flick down drops once; a slow drag down soft-drops
- [ ] A new piece during a drag keeps following the same finger
- [ ] Buttons layout: ◀ ▶ repeat when held, ⟳ ▼ ⤓ work; left-handed mirrors the order
- [ ] Clears flash, sand bursts, chain text reads CLEAR / DOUBLE CHAIN / … / LEMON STORM

## Dealing feel (the Director)
- [ ] Colours look random over a few minutes: no colour three times in a row, no colour dominating
- [ ] Near the top, a helpful colour or shape comes often enough to recover, without it being obvious
- [ ] After a clear, the next pieces often allow another; chains of 2–3 happen in normal play
- [ ] On a low, clean board the game does not feel scripted or too easy

## Feedback
- [ ] Haptics: light on turn and place, heavy on drop, success on big chains and new best, error on game over; off when disabled
- [ ] Music and effects volumes independent; Mute persists across restarts
- [ ] Reduce motion: no screen shake, far fewer particles

## Performance (release build, profiler attached)
- [ ] 60 fps sustained in a 10-minute run; on a 120 Hz device, 120 fps with a smooth piece
- [ ] No frame over 33 ms during big chains
- [ ] Memory stable over 10 minutes (no growth)
- [ ] Battery and thermal acceptable after 20 minutes

## Scores
- [ ] Set a name with the native prompt; it appears on the results and the scores screen
- [ ] A new best shows NEW BEST with confetti and survives an app restart
