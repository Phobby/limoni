package core

// EventKind is what happened during a step, for sound, particles and haptics.
// Events never change the rules; dropping them changes nothing but feedback.
type EventKind uint8

const (
	EvNone EventKind = iota
	EvStart
	EvSpawn
	EvMove
	EvTurn
	EvLand     // a piece let down became sand
	EvDrop     // a hard-dropped piece became sand
	EvTrickle  // sand is running (N grains moved this step)
	EvClear    // a run started to flash (N grains; Base/Chain/Points when scored)
	EvVanish   // a flash ended and its grains are gone
	EvLevelUp  // N is the new level
	EvGameOver //
)

// Event is one thing that happened.
type Event struct {
	Kind        EventKind
	N           int
	Base, Chain int
	Points      int
	X, Y        int // grains (land/drop: the piece's box)
	Piece, Rot  int
	Colour      uint8
}

func (s *Sim) emit(e Event) {
	if s.nev == len(s.ev) { // full: keep the newest, drop the oldest
		s.evLo = (s.evLo + 1) % len(s.ev)
		s.nev--
	}
	s.ev[(s.evLo+s.nev)%len(s.ev)] = e
	s.nev++
}

// PollEvent takes the oldest unread event; ok is false when there is none.
func (s *Sim) PollEvent() (e Event, ok bool) {
	if s.nev == 0 {
		return Event{}, false
	}
	e = s.ev[s.evLo]
	s.evLo = (s.evLo + 1) % len(s.ev)
	s.nev--
	return e, true
}
