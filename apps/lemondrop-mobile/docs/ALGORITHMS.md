# Piece generation, scoring and difficulty

All generators are written for this game. The Block Blast reverse-engineering
reports (research workspace: `reports/piece-generation.md`,
`board-evaluation.md`, `combo-scoring.md`, `algorithm-pseudocode.md`,
`unverified-hypotheses.md`, `t1/README.md`, `engagement-and-difficulty.md`)
were used as a reference for *principles only*. No code, data tables, assets
or names were copied.

## What was taken from the research, and what was left out

| Principle in the research (evidence level there) | Here | Where |
|---|---|---|
| Seeded LCG-style RNG, deterministic for a seed (emulation-verified) | Yes: xorshift64* seeded per run; every generator takes randomness only from the run's RNG | `core.RNG`, `gen/*` |
| Board-aware filtering: only offer pieces that fit (observed) | Yes, for Zest Blocks | `gen/zest.go` |
| Bounded look-ahead over placements with line clears in between (observed, T1-verified) | Yes: DFS over all 6 orders of the tray with clears, node budget | `Zest.solvable` |
| Fallback when sampling fails (observed: small-piece fallback) | Yes: exhaustive search over small pieces | `Zest.Deal` |
| Score-bucketed weights (observed; values unknown) | **Changed:** weights depend only on board fill, and the effect is visible in settings | `ZestProfile.Crowd` |
| "Hard" families: no single-piece clear, one valid order (T1-measured) | **Left out** | — |
| Avoid full board clears (`selectBlockAvoidClearBoard`) | **Left out** — rationing rewards is invisible manipulation | — |
| Break clearing streaks (`selectBlockStopRefreshScoreIOS`) | **Left out** | — |
| Hidden A/B difficulty switches chosen by a server | **Left out** — profiles are chosen by the player | Settings |

## Falling-sand mode

### Classic
The original 7-bag with a random colour (`core.ClassicNext`), unchanged.
Classic runs go to the shared `/drop/scores` board, whose server recomputes
the score from the clear log; `core` reproduces the terminal game bit for bit
(`TestParityWithTerminalGame`).

### Fair Drop (`gen/fairdrop.go`)
- **Fairness that is never broken:** every aligned window of seven pieces holds
  each shape exactly once, exactly like the classic bag
  (`TestFairDropKeepsBagFairnessAndColourBalance`). Fair Drop only chooses
  *which* remaining shape of the bag comes next, and its colour.
- **When it helps:** with probability `Calm` on an empty board, ramping
  linearly to `Assist` as the stack reaches `DangerAt` of the board height.
  Otherwise the piece is dealt blindly, as Classic does.
- **How it judges a candidate:** for every shape in the bag × 4 colours, every
  rotation × every half-block position (bounded: ≤ 7×4×4×~20 evaluations),
  the landing is computed on a grain-column height map. It is valued by how
  much it widens the band of its colour near the surface (what a wall-to-wall
  run needs), minus stack height² and the gaps it leaves. Sand slumps after
  landing, so this is an approximation; exact grain simulation per candidate
  would cost ~100× more.
- **Choosing:** a softmax over candidate values with `Temperature`, so the
  best piece is likely but not certain (keeps variety).
- **Profiles:** Relaxed (Calm 0.35 → Assist 0.85 by 55% height), Balanced
  (0.1 → 0.5 by 70%).
- **Measured effect:** the same fixed bot survives longer with Relaxed than with
  Classic (`TestFairDropHelpsTheSamePlayer`: 592 vs 527 pieces over 12 seeded
  runs). Cost: 121 µs per piece on an Apple M2, 0 allocations.
- **Leaderboard:** local only, because piece order differs from Classic.

## Zest Blocks (`blocks/`, `gen/zest.go`)

Rules (also on the How to play screen):

```
placing a piece           +1 a cell
clearing L lines at once  80 × L × (L + Z) × combo     Z = zest lines (one colour)
emptying the board        +300
combo                     +1 per clearing placement; three placements without a clear end it
game over                 no piece left in the tray fits anywhere
```

Dealing (`Zest.Deal`):
1. Candidates are the shapes that fit somewhere now.
2. Weights: mid-size pieces 1.0, single cells 0.6, 6–8 cells 0.6, 3×3 0.35;
   pieces of 5+ cells × `max(0.05, 1 − Crowd × fill)`.
3. Challenge: three weighted draws, no promise.
4. Standard / Relaxed: sample trays until one is **solvable** (all three placed
   in some order with clears between; bounded by `Budget` nodes). Relaxed
   compares `Samples` solvable trays and prefers the one that can clear the
   most lines right away.
5. Fallback: exhaustive search over pieces of ≤ 3 cells; a run ends only when
   no such tray exists.

Measured (`gen/zest_test.go`): 400/400 random boards that admitted a solvable
small tray got a solvable tray (Standard and Relaxed); Relaxed deals 5% big
pieces on 55% full boards vs 9% for Challenge; slowest deal 17 µs.

## Determinism
- Same seed + same inputs ⇒ same run and score (`TestSameSeedSameRun`).
- Frame rate does not change the run: the simulation advances in fixed
  1/60 s steps; 60 and 120 fps give identical states
  (`TestUpdateIsFrameRateIndependent`).
- Effects, sound and haptics read events and never change rules.
