# Device test checklist

Tick each item on at least one small phone, one large phone and one
high-refresh device per platform. Status today: iOS simulator smoke-tested
(menu, Classic, Zest render and run); no physical device yet.

## Install and lifecycle
- [ ] Cold start shows the splash, then the menu, in under 2 s
- [ ] Home / app switcher during a run, then back: the run is paused and nothing is lost
- [ ] Phone call / Control Center / notification shade during a run: paused, audio resumes after
- [ ] Kill the app during a run, relaunch: settings and boards are intact
- [ ] Airplane mode: Classic results say "offline — kept on this device"; no hang

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

## Zest Blocks
- [ ] Dragged piece rides above the finger; snapping is predictable at the board edges
- [ ] Invalid spot shows red and the piece returns to the tray
- [ ] Lines that will clear glow before release
- [ ] Game over appears only when no tray piece fits

## Feedback
- [ ] Haptics: light on turn and place, heavy on drop, success on big chains and new best, error on game over; off when disabled
- [ ] Music and effects volumes independent; Mute persists across restarts
- [ ] Reduce motion: no screen shake, far fewer particles

## Performance (release build, profiler attached)
- [ ] 60 fps sustained in a 10-minute Classic run; on a 120 Hz device, 120 fps with a smooth piece
- [ ] No frame over 33 ms during big chains
- [ ] Memory stable over 10 minutes (no growth)
- [ ] Battery and thermal acceptable after 20 minutes

## Leaderboard
- [ ] Set a name with the native prompt; it appears on the results and the board
- [ ] A Classic run is accepted by the world board; Fair Drop and Zest never post
