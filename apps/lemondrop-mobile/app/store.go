package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Save data is one versioned JSON file in the platform's data directory.
// Version 1 is the first mobile format. Legacy data from the terminal game
// (limoni/lemondrop.json: a name and a board scored with the same rules) is
// imported into the Classic board once, on first run.

const saveVersion = 1

// Mode keys a board in the save file. There is one mode; the key keeps the
// format open for more.
type Mode string

const modeDrop Mode = "drop"

const boardSize = 20

type entry struct {
	Name   string `json:"name"`
	Score  int    `json:"score"`
	Clears int    `json:"clears"`
	Level  int    `json:"level"`
	Secs   int    `json:"secs"`
	At     int64  `json:"at,omitempty"`
}

type settings struct {
	SfxVol     float64 `json:"sfxVol"`
	MusicVol   float64 `json:"musicVol"`
	Muted      bool    `json:"muted"`
	Haptics    bool    `json:"haptics"`
	ReduceMove bool    `json:"reduceMotion"`
	Buttons    bool    `json:"buttons"` // on-screen buttons instead of gestures
	LeftHanded bool    `json:"leftHanded"`
}

func defaultSettings() settings {
	return settings{SfxVol: 0.9, MusicVol: 0.35, Haptics: true}
}

type saveFile struct {
	Version  int              `json:"version"`
	Name     string           `json:"name"`
	Settings settings         `json:"settings"`
	Boards   map[Mode][]entry `json:"boards"`
	Imported bool             `json:"importedLegacy"`
}

type store struct {
	path, legacy string
	data         saveFile
}

func newStore(dir string) *store {
	s := &store{data: saveFile{Version: saveVersion, Settings: defaultSettings(), Boards: map[Mode][]entry{}}}
	if dir == "" {
		return s
	}
	s.path = filepath.Join(dir, "lemondrop-mobile.json")
	s.legacy = filepath.Join(dir, "lemondrop.json")
	s.load()
	return s
}

func (s *store) load() {
	if b, err := os.ReadFile(s.path); err == nil {
		var f saveFile
		if json.Unmarshal(b, &f) == nil && f.Version >= 1 && f.Version <= saveVersion {
			if f.Boards == nil {
				f.Boards = map[Mode][]entry{}
			}
			s.data = f
		}
	}
	if !s.data.Imported {
		s.importLegacy()
	}
}

// importLegacy reads the terminal game's save (same scoring rules).
func (s *store) importLegacy() {
	s.data.Imported = true
	b, err := os.ReadFile(s.legacy)
	if err != nil {
		return
	}
	var old struct {
		Name  string  `json:"name"`
		Board []entry `json:"board"`
	}
	if json.Unmarshal(b, &old) != nil {
		return
	}
	if s.data.Name == "" {
		s.data.Name = cleanName(old.Name)
	}
	for _, e := range old.Board {
		e.Name = cleanName(e.Name)
		s.add(e)
	}
	s.save()
}

// save writes atomically: a temporary file renamed over the old one.
func (s *store) save() {
	if s.path == "" {
		return
	}
	b, err := json.MarshalIndent(s.data, "", " ")
	if err != nil || os.MkdirAll(filepath.Dir(s.path), 0o755) != nil {
		return
	}
	tmp := s.path + ".tmp"
	if os.WriteFile(tmp, b, 0o644) == nil {
		_ = os.Rename(tmp, s.path)
	}
}

// add puts e on the board and returns its place (1-based) or 0.
func (s *store) add(e entry) int {
	m := modeDrop
	b := append(s.data.Boards[m], e)
	sort.SliceStable(b, func(i, j int) bool { return b[i].Score > b[j].Score })
	rank := 0
	for i := range b {
		if b[i] == e {
			rank = i + 1
			break
		}
	}
	if len(b) > boardSize {
		b = b[:boardSize]
	}
	if rank > boardSize {
		rank = 0
	}
	s.data.Boards[m] = b
	return rank
}

func (s *store) board() []entry { return s.data.Boards[modeDrop] }

func (s *store) best() int {
	if b := s.board(); len(b) > 0 {
		return b[0].Score
	}
	return 0
}

// cleanName keeps what the terminal game's leaderboard accepts (apps/lemondrop).
func cleanName(in string) string {
	const nameMax = 12
	var b strings.Builder
	n := 0
	for _, r := range strings.TrimSpace(in) {
		if n == nameMax {
			break
		}
		ok := r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || r >= '0' && r <= '9' ||
			r == ' ' || r == '-' || r == '_' || r == '.' || strings.ContainsRune("ÇĞİÖŞÜçğıöşü", r)
		if ok && (r != ' ' || n > 0) {
			b.WriteRune(r)
			n++
		}
	}
	return strings.TrimSpace(b.String())
}
