# Performance

Measured on an Apple M2 MacBook (macOS, Metal) with Go 1.27.1 and
Ebitengine v2.10.6. **No phone has been profiled yet**: see the device
checklist.

## Rules engine (`go test ./core ./gen -bench .`)

| Benchmark | Time | Allocations |
|---|---|---|
| `core` step on a two-thirds-full, fully awake board (60×108 grains) | 27.6 µs | 0 |
| Fair Drop piece choice (once per piece, worst case: always evaluating) | 121 µs | 0 |
| Zest deal, Relaxed profile | 4.8 µs | — |

`TestStepsAllocateNothing` fails the build if a simulation step allocates.

## Frames (`cmd/lemondrop-mobile -bench`)

CPU time is `Update + Draw` on the CPU; GPU work is asynchronous and not
included. 393×852 dp window at 2× (786×1704 px).

| Scenario | CPU/frame mean | p99 | max | FPS |
|---|---|---|---|---|
| Classic, 3× speed (busy sand, clears, particles) | 0.49–0.56 ms | 1.5–2.0 ms | 5.9–10 ms | 60 |
| Zest Blocks | 2.7 ms | 8.8 ms | 32 ms (first frames: sprite cache warm-up) | 60 |
| Menu with the attract board | 3.6 ms | 7.7 ms | 25 ms | 60 |

Before the sprite cache, Classic took 2.6 ms a frame. Since then, blocks,
cells and ghosts are drawn with vectors once per size and colour, and are
blitted after that.

### Allocations
- **Our code:** about zero bytes per frame in steady state. An allocation
  profile of a 1 440-frame Classic run attributes 6.6 kB *in total* to our
  package (HUD number strings); everything else is start-up (`newFX`, 0.75 MB).
- **Ebitengine's macOS Metal driver:** about 93 kB and 3 200 mallocs per frame,
  mostly `purego/objc.ID.Send` and `metal.adjustUniformVariablesLayout`. This
  is per GPU command in the desktop driver and outside this project. iOS and
  Android use different driver paths and must be measured on devices.
- GC ran 16–18 times in a 24 s run on the desktop; no frame took more than
  10 ms in Classic.

## Design choices that keep frames cheap
- Sand is one 60×108 texture written once per frame (6 480 texels) and drawn
  with one shader call, never one draw per grain.
- Particles: fixed pool of 3 000, one `DrawTriangles` per blend mode, vertex
  and index slices reused.
- Shader uniforms live in slices that are allocated once.
- Expensive board evaluation runs only when a piece is dealt, never in Draw.
- The app pauses when it loses focus and saves on suspend.

## How to measure

```
go test ./core ./gen -run XXX -bench . -benchmem
go run ./cmd/lemondrop-mobile -start drop -speed 3 -frames 1500 -bench -mute -board off
go run ./cmd/lemondrop-mobile -start zest -frames 600 -bench -memprofile zest.mem
go tool pprof -top -sample_index=alloc_space zest.mem
```

On devices, use Instruments (Time Profiler, Allocations, Metal System Trace)
or the Android Studio profiler with the release build.
