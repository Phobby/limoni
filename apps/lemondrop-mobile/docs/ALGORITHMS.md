# Piece dealing: the Director

Lemon Drop Mobile has one mode: falling blocks that crumble into sand. The
rules and scoring are the terminal game's, ported bit for bit (`core`,
`TestParityWithTerminalGame`). What differs is how pieces are dealt: instead of
a blind shuffle with random colours, the **Director** (`gen/director.go`)
chooses each colour, and the order of shapes within the 7-bag, from how the
run is going.

## Goals
1. Not too easy and not too hard. The game must still end.
2. Near defeat, small, believable help that gives a way out.
3. After a clear, chances to chain and keep the streak going.
4. Help that never shows. Colours must look random: no long single-colour
   streaks, balanced shares.
5. Never deal to make the player lose. Every seven pieces still hold each
   shape exactly once.

## How it decides, every piece

**1. Read the run.**
- *danger* = smoothstep of the stack height, from 35% to 82% of the board.
- *stall* = pieces since the last clear ÷ 10, capped at 1.
- *heat* jumps to ≈0.6 (more for several clears at once) when a piece leads to
  a clear, then decays ×0.6 a piece.

**2. Read the board.**
- A height map of the surface, one entry per grain column.
- For each colour, the columns where it lies within 3 blocks of the surface.
  A wall-to-wall run needs a colour in every column.
- The piece about to fall is "placed" where a sensible player would put it,
  so the next deal is judged on the board it will actually meet.

**3. How much to help.**

```
assist = 0.12 + 0.68·danger + 0.20·stall + 0.50·heat
assist ≤ 0.55 + 0.35·danger      (≤ 0.55 on a low stack, ≤ 0.90 near the top)
```

With probability `1 − assist` the piece is dealt blind, as the classic bag
does, except that a third piece of one colour in a row is never dealt.

**4. Choose.** When it helps, every shape left in the bag × 4 colours is
valued at its best landing. All rotations × half-block positions are tried, at
most 7 × 4 × 4 × 20 placements:

| Term | Weight | Meaning |
|---|---|---|
| progress | 0.7 + 0.9·danger | columns this colour newly reaches near the surface |
| complete | 0.5 + 1.1·danger + 0.9·heat | 1 if the colour then spans wall to wall (a clear), 0.4 if one block short |
| closeness | 0.6·heat | how close the colour is to spanning: keeps a chain going |
| height | −(0.5 + 1.6·danger)·h² | how high the piece would sit |
| gaps | −(0.2 + 0.4·danger) | holes left under it |
| naturalness | −5 for a third same colour in a row; −0.18 for a second; −2·(share−0.375) above share 0.375 in the last 8 | keeps colours looking random |

The pick is a softmax over these values with temperature 0.22 − 0.10·danger.
The best option is likely but not certain, so help reads as luck.

**5. Labels for tests.** Each deal is labelled *Blind*, *Flow*, *Chain*
(heat > 0.25) or *Rescue* (danger > 0.55). The labels are for tests only and
never shown to the player.

## Measured (`go test ./gen -v`)
The same fixed bot plays 12 seeded runs with blind dealing and with the
Director:

| | Blind (classic bag, random colours) | Director |
|---|---|---|
| pieces survived | 527 | 681 (+29%) |
| clears | 14 | 35 (×2.5) |
| a clear followed by another within 3 pieces (40 runs) | 6% | 26% |
| runs that still ended (cap 600 pieces) | 12/12 | 12/12 |
| same colour as the previous piece | ≈25% (chance) | 28% |
| longest run of one colour | — | 2 |

Other checks:
- **Help grows with danger:** assistance is 0.12 with the stack at 20% and
  0.80 at 80% (`TestDirectorHelpsMoreNearTheTop`).
- **The bag stays fair:** every aligned window of seven pieces holds each
  shape once (`TestDirectorKeepsTheBagFair`).
- **Deal mix over the 12 runs:** 35% blind, the rest flow, chain or rescue
  (`TestDirectorIsHelpfulButNotTrivial`).
- **Cost:** 62 µs per piece on an Apple M2, 0 allocations
  (`BenchmarkDirectorNext`).

The bot plays poorly, so it spends much of its time in danger. A human who
plays better meets the Rescue branch less often and gets less help.

## Relation to the Block Blast research
The research reports (research workspace: `reports/`) were a reference for
principles, not code:

- **Used:**
  - a seeded RNG;
  - board-aware evaluation of candidates over legal placements;
  - assistance that ramps with the player's situation;
  - a softmax over candidates so help does not look scripted.
- **Not used:**
  - the research's hard families;
  - full-clear avoidance;
  - streak breaking;
  - hidden difficulty spikes.

  The Director only ever helps, or stays blind.

## Determinism and fairness
- **Same seed and same inputs give the same run and score**
  (`TestDirectorIsSeeded`, `TestSameSeedSameRun`).
- **Frame rate does not change the run:** the simulation steps at a fixed
  1/60 s (`TestUpdateIsFrameRateIndependent`).
- **Local scores only:** piece dealing differs from the terminal game's
  classic bag, so runs are kept on this device's board and are not sent to
  the terminal game's shared leaderboard.
