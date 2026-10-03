package run

import (
	"bufio"
	"encoding/json"
	"github.com/KakkoiDev/agame/world"
	"os"
	"path/filepath"
)

type Store struct{ Dir string }

// Save replaces world.json atomically (temp file + rename) so a crash never leaves a truncated, half-written turn.
func (s Store) Save(w *world.World) error {
	if err := os.MkdirAll(s.Dir, 0755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(w, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(s.Dir, ".world-*.json")
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
		err = os.Rename(tmp, filepath.Join(s.Dir, "world.json"))
	}
	if err != nil {
		os.Remove(tmp)
	}
	return err
}
func (s Store) Load() (*world.World, error) {
	b, err := os.ReadFile(filepath.Join(s.Dir, "world.json"))
	if err != nil {
		return nil, err
	}
	var w world.World
	if err = json.Unmarshal(b, &w); err != nil {
		return nil, err
	}
	return &w, nil
}
func (s Store) Append(events []world.Event) error {
	if err := os.MkdirAll(s.Dir, 0755); err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(s.Dir, "events.jsonl"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	bw := bufio.NewWriter(f)
	enc := json.NewEncoder(bw)
	for _, e := range events {
		if err = enc.Encode(e); err != nil {
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
