# Lemon Drop Mobile — audit and architecture

## 1. Audit of `apps/lemondrop` (terminal game)

| # | System | Where | How it works | Mobile verdict |
|---|---|---|---|---|
| 1 | Game loop | `main.go` → `limoni.NewApp(...).Run`, `game.update` | Terminal frame callback; `update(elapsed)` accumulates time and runs fixed `tick`s at 60 Hz (`dt = 1/60`, clamp 0.1 s) | **Reuse the scheme** (fixed step, accumulator). Ebitengine's `Update` already runs at a fixed TPS; we keep our own accumulator for determinism independent of TPS. |
| 2 | Simulation state | `game` struct in `game.go` | One value with fixed arrays (`sand`, `vel`, `frac`, `mark`, `stack`, `clr`, `awake`), plus UI, leaderboard, renderer scratch | **Rules reused, struct split.** Rules → `core.Sim`; UI/leaderboard/render state → `app`. |
| 3 | Rendering | `render.go` | Half-block characters, one grain per pixel, palette of 4 colours × 4 shades, background lemon drawn once per size and re-shaded as it turns | **Not reusable** for mobile (character cells). Palette, lemon geometry (`lemonAt`) and shading rules are ported to GPU drawing. |
| 4 | Particle physics | `game.fall` | Cellular sand: per-grain speed (`vel`, 1/32 grain), sub-cell progress (`frac`), straight down else diagonal at random, slower slides with hesitation, alternating sweep direction, only "awake" rows visited | **Ported unchanged** (identical constants and RNG). |
| 5 | Input | `main.go` `key`/`mouse` | Keys with DAS (`dasDelay` 0.16 s, `dasRate` 1/40 s), soft drop, hard drop, mouse for pause/leaderboard | **Replaced** by touch gestures (drag/tap/swipe) + keyboard on desktop. Rules-level actions (`Shift`, `Turn`, `HardDrop`, `SoftDrop`) are kept. |
| 6 | Frame timing | `update` | Accumulator, `min(elapsed, 0.1)` | Same, plus render interpolation of the falling piece. |
| 7 | Collision | `game.fits` | Piece blocks (b×b grains) vs walls/floor/any grain; above the top is open; wall kicks in half blocks | **Ported unchanged.** |
| 8 | Colour connectivity / clear | `findClear` | Iterative flood fill (8-neighbourhood) from every grain on the left wall; a component that reaches the right wall flashes for 0.45 s then vanishes; rows above are woken | **Ported unchanged.** |
| 9 | Combo / scoring | `findClear`, `score.go`, `apps/scoreboard/board/drop.go` | `base = round(grains×10 / b²)`; `level = 1 + clears/4`; `pts = base × level × chain`; chain = clears since the last landing; hard drop 2/row, soft drop 1/row | **Ported unchanged** — the shared server recomputes scores from `[base, chain]` logs, so the mobile game must not change these rules. |
| 10 | Audio | `sound.go` + `sound_system.go` (oto) | Clips synthesised at start-up (tones, filtered noise), mixed on a goroutine into 16-bit stereo; `play` never blocks | **Synth ported**; output moved to Ebitengine `audio` (works on iOS/Android). |
| 11 | Persistence / leaderboard | `score.go`, `store_*.go`, `remote*.go` | JSON file in the user config dir (atomic rename); `/drop/scores` GET/POST off-frame on a goroutine, polled from the step without blocking | **Protocol reused**; file location injected by the native shell on mobile (sandboxed data dir). |
| 12 | Performance | throughout | No allocation per step or frame (tests enforce it), awake-row sweep, flood fill with stamps instead of clearing marks | **Kept as a rule** in `core` (enforced by tests and benchmarks). |

### Can Limoni itself run on mobile?

Limoni renders to a character-cell buffer and diffs it to a terminal (or xterm.js in the browser). Reusing it on a phone would mean drawing terminal output into a WebView or emulating a terminal — low resolution (a grain per half cell), no GPU effects, no native touch. **Decision:** keep Limoni for the terminal game untouched and implement a native GPU frontend with **Ebitengine v2.10** (pure Go, Metal on iOS, OpenGL ES on Android, `ebitenmobile bind` for both). The simulation is shared in spirit and in rules; see §3 on duplication.

## 2. Module layout (`apps/lemondrop-mobile`, its own Go module)

```
core/      falling-sand rules (ported from apps/lemondrop/game.go): Sim, events, run log
gen/       the Director: adaptive dealing of colours and bag order (docs/ALGORITHMS.md)
app/       Ebitengine game: screens, layout, input, rendering, particles, audio, haptics,
           settings, persistence, local scores
cmd/lemondrop-mobile/   desktop runner (development and screenshots)
mobile/    package for `ebitenmobile bind` (iOS .xcframework, Android .aar) + native bridges
ios/       Xcode project (XcodeGen spec + Swift shell)
android/   Gradle project (Kotlin shell)
docs/      this file, algorithms, build, performance, device checklist
```

`core` and `gen` import nothing from Ebitengine, so their tests run anywhere and deterministically.

## 3. Decisions

1. **Separate module, original untouched.** `apps/lemondrop` is `package main`; it cannot be imported. Refactoring it into a library would change the shipped terminal game. The rules were therefore ported into `core` with identical constants, RNG and algorithms, and the original tests were ported as regression tests (`core/core_test.go`). *Follow-up:* once accepted, `apps/lemondrop` can import `core` and the duplication disappears.
2. **Fixed grain size b = 6.** The terminal picks b from the window (2–6). On a phone the board always has room, and 6 is the largest the shared scoreboard accepts (`dropMaxB = 6`). Scores are per block's worth of grains, so they do not depend on b.
3. **Fixed-timestep simulation, interpolated rendering.** `core` steps at 60 Hz regardless of display rate. The renderer interpolates only the falling piece's sub-grain position; sand is drawn as simulated (grains move at most one cell per step).
4. **Visuals never feed back into rules.** `core` emits events (`Landed`, `Cleared`, `Chain`, `LevelUp`, `GameOver`, …) into a fixed ring buffer; the app turns them into particles, sound and haptics. Score is computed only in `core`.
5. **Sand on the GPU as one texture.** 60×108 grains are written into a 60×108 RGBA texture each frame (6 480 pixels) and drawn with one Kage shader that adds per-grain bevel and lighting. No per-grain draw calls.
6. **Particles batched.** A fixed pool; all live particles are one `DrawTriangles` call per blend mode.
7. **Resolution independence.** Layout works in *dp* (`Layout` returns device pixels = dp × device scale). The board is fitted into the safe area with an integer-free scale; touch coordinates are converted back to board grains through the same transform.
8. **One mode, local scores.** The game has a single mode: falling sand. Earlier drafts had Classic, Fair Drop and a drag-and-place mode; they were cut in favour of one mode whose pieces are dealt by the Director. Scoring is unchanged, but dealing is not the classic bag, so runs are not sent to the terminal game's shared `/drop/scores` board; the device keeps its own top 20.
9. **Native bridges are thin.** Haptics, text input (player name), data directory and safe-area insets come from Swift/Kotlin through a small exported Go interface in `mobile/`.
