// Package run stores one universe on disk:
//
//	run.json        run header: ruleset, seed, initial-state hash, roster, budget
//	initial.json    S(0), the replay starting point
//	world.json      the current authoritative state
//	turns.jsonl     one world.TurnResult per turn: submitted, accepted and
//	                rejected orders, events and the resulting state hash
//	events.jsonl    the append-only event log
//	decisions.jsonl one decision record per ruler and turn
//	reflections.jsonl one record per reflection phase offered
//	jikko.json      the rulers' Jikko store (identities, documents, history)
//	result.json     end condition and final standings, once the run ends
package run

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/KakkoiDev/agame/world"
)

// File names inside a run directory.
const (
	WorldFile       = "world.json"
	InitialFile     = "initial.json"
	HeaderFile      = "run.json"
	TurnsFile       = "turns.jsonl"
	EventsFile      = "events.jsonl"
	DecisionsFile   = "decisions.jsonl"
	ReflectionsFile = "reflections.jsonl"
	ResultFile      = "result.json"
	JikkoFile       = "jikko.json"
)

type Store struct{ Dir string }

// RosterEntry names the decision provider of one ruler.
type RosterEntry struct {
	Empire string         `json:"empire"`
	Name   string         `json:"name"`
	Agent  string         `json:"agent"`
	Config map[string]any `json:"config,omitempty"`
}

// Header is the reproducibility record of a run (spec/benchmark.md,
// Reproducible universe).
type Header struct {
	Ruleset       string        `json:"ruleset"`
	PromptVersion string        `json:"prompt_version,omitempty"`
	Seed          int64         `json:"seed"`
	InitialHash   string        `json:"initial_hash"`
	Roster        []RosterEntry `json:"roster,omitempty"`
	Budget        any           `json:"budget,omitempty"`
	// JikkoRevision is the Jikko store revision the run started from; -1
	// when the run has no Jikko store.
	JikkoRevision int `json:"jikko_start_revision"`
}

// Save replaces world.json atomically (temp file + rename) so a crash never leaves a truncated, half-written turn.
func (s Store) Save(w *world.World) error { return s.SaveJSON(WorldFile, w) }

func (s Store) Load() (*world.World, error) {
	var w world.World
	if err := s.LoadJSON(WorldFile, &w); err != nil {
		return nil, err
	}
	return &w, nil
}

// Create starts a new run from S(0): it refuses to overwrite an existing
// universe, then writes the initial state, the current state and the header.
func (s Store) Create(w *world.World, h Header) error {
	if _, err := os.Stat(filepath.Join(s.Dir, WorldFile)); err == nil {
		return fmt.Errorf("%s already contains a universe", s.Dir)
	}
	h.Ruleset = world.RulesetVersion
	h.Seed = w.Seed
	h.InitialHash = world.StateHash(w)
	if err := s.SaveJSON(InitialFile, w); err != nil {
		return err
	}
	if err := s.SaveJSON(HeaderFile, h); err != nil {
		return err
	}
	return s.Save(w)
}

// Commit records a resolved turn: the turn log, events and decisions are
// appended first and the new world is saved last, so world.json never runs
// ahead of the log that explains it.
func (s Store) Commit(w *world.World, res world.TurnResult, decisions ...any) error {
	if err := s.AppendJSONL(TurnsFile, res); err != nil {
		return err
	}
	if err := s.Append(res.Events); err != nil {
		return err
	}
	if err := s.AppendJSONL(DecisionsFile, decisions...); err != nil {
		return err
	}
	return s.Save(w)
}

// Turns reads the turn log.
func (s Store) Turns() ([]world.TurnResult, error) {
	var out []world.TurnResult
	err := s.ReadJSONL(TurnsFile, func(b []byte) error {
		var t world.TurnResult
		if err := json.Unmarshal(b, &t); err != nil {
			return err
		}
		out = append(out, t)
		return nil
	})
	return out, err
}

// SaveJSON writes name atomically as indented JSON.
func (s Store) SaveJSON(name string, v any) error {
	if err := os.MkdirAll(s.Dir, 0755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(s.Dir, ".tmp-*.json")
	if err != nil {
		return err
	}
	tmp := f.Name()
	_, err = f.Write(b)
	if err == nil {
		err = f.Sync()
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Chmod(tmp, 0644)
	}
	if err == nil {
		err = os.Rename(tmp, filepath.Join(s.Dir, name))
	}
	if err != nil {
		os.Remove(tmp)
	}
	return err
}

// LoadJSON decodes name into v.
func (s Store) LoadJSON(name string, v any) error {
	b, err := os.ReadFile(filepath.Join(s.Dir, name))
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}

// Append adds events to the append-only event log.
func (s Store) Append(events []world.Event) error {
	items := make([]any, len(events))
	for i, e := range events {
		items[i] = e
	}
	return s.AppendJSONL(EventsFile, items...)
}

// AppendJSONL appends one JSON line per item.
func (s Store) AppendJSONL(name string, items ...any) error {
	if err := os.MkdirAll(s.Dir, 0755); err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(s.Dir, name), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	bw := bufio.NewWriter(f)
	enc := json.NewEncoder(bw)
	for _, x := range items {
		if err = enc.Encode(x); err != nil {
			break
		}
	}
	if err == nil {
		err = bw.Flush()
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	return err
}

// ReadJSONL calls f with every non-empty line of name. A missing file has no
// lines.
func (s Store) ReadJSONL(name string, f func([]byte) error) error {
	b, err := os.ReadFile(filepath.Join(s.Dir, name))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	for i, line := range bytes.Split(b, []byte("\n")) {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		if err := f(line); err != nil {
			return fmt.Errorf("%s line %d: %w", name, i+1, err)
		}
	}
	return nil
}

// Tail returns up to the last n non-empty lines of name, oldest first,
// reading backwards from the end so large logs stay cheap. A missing file
// has no lines.
func (s Store) Tail(name string, n int) ([][]byte, error) {
	f, err := os.Open(filepath.Join(s.Dir, name))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return nil, err
	}
	const block = 64 << 10
	var buf []byte
	pos := st.Size()
	for pos > 0 && bytes.Count(bytes.TrimRight(buf, "\n"), []byte("\n")) < n {
		size := int64(block)
		if pos < size {
			size = pos
		}
		pos -= size
		chunk := make([]byte, size)
		if _, err := f.ReadAt(chunk, pos); err != nil {
			return nil, err
		}
		buf = append(chunk, buf...)
	}
	var lines [][]byte
	for _, l := range bytes.Split(buf, []byte("\n")) {
		if len(bytes.TrimSpace(l)) > 0 {
			lines = append(lines, l)
		}
	}
	if pos > 0 && len(lines) > 0 { // the first line may be cut
		lines = lines[1:]
	}
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return lines, nil
}
