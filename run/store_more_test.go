package run

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/KakkoiDev/agame/world"
)

var names = []string{"A", "B", "C", "D", "E", "F", "G", "H"}

func TestSaveLoadPreservesPlayedWorld(t *testing.T) {
	s := Store{Dir: filepath.Join(t.TempDir(), "nested", "run")}
	w, _ := world.Generate(3, names)
	p := w.Planets[w.Empires["e00"].HomeworldID]
	p.Construction = &world.Queue{Kind: "metal_mine", Level: 2, Required: 2, Paid: world.Resources{Metal: 120}}
	w.Fleets["f000000"] = &world.Fleet{ID: "f000000", OwnerID: "e00", SystemID: p.SystemID, Ships: world.Ships{"scout": 1}, Route: []string{p.SystemID}}
	if _, err := world.ResolveTurn(w, map[string][]world.Order{"e00": {{EmpireID: "e00", Type: "message", Target: "e01", Params: map[string]any{"body": "hi"}}}}); err != nil {
		t.Fatal(err)
	}
	if err := s.Save(w); err != nil {
		t.Fatal(err)
	}
	if err := s.Save(w); err != nil { // overwrite
		t.Fatal(err)
	}
	got, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	a, _ := json.Marshal(w)
	b, _ := json.Marshal(got)
	if string(a) != string(b) {
		t.Fatal("world changed across save/load")
	}
	entries, _ := os.ReadDir(s.Dir)
	if len(entries) != 1 || entries[0].Name() != "world.json" {
		t.Fatalf("unexpected files after save: %v", entries)
	}
}

func TestLoadErrors(t *testing.T) {
	d := t.TempDir()
	if w, err := (Store{Dir: d}).Load(); err == nil || w != nil {
		t.Fatalf("missing run: w=%v err=%v", w, err)
	}
	if err := os.WriteFile(filepath.Join(d, "world.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if w, err := (Store{Dir: d}).Load(); err == nil || w != nil {
		t.Fatalf("corrupt run: w=%v err=%v (a half-decoded world must not be returned)", w, err)
	}
}

func TestAppendWritesJSONLInOrder(t *testing.T) {
	s := Store{Dir: t.TempDir()}
	if err := s.Append([]world.Event{{Turn: 1, Type: "a"}, {Turn: 1, Type: "b"}}); err != nil {
		t.Fatal(err)
	}
	if err := s.Append(nil); err != nil {
		t.Fatal(err)
	}
	if err := s.Append([]world.Event{{Turn: 2, Type: "c", EmpireID: "e00"}}); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(filepath.Join(s.Dir, "events.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var got []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var e world.Event
		if err := json.Unmarshal(sc.Bytes(), &e); err != nil {
			t.Fatalf("line %q: %v", sc.Text(), err)
		}
		got = append(got, e.Type)
	}
	if len(got) != 3 || got[0] != "a" || got[1] != "b" || got[2] != "c" {
		t.Fatalf("events %v", got)
	}
}

func TestAppendReportsWriteFailure(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("uses /dev/full")
	}
	if _, err := os.Stat("/dev/full"); err != nil {
		t.Skip("no /dev/full")
	}
	d := t.TempDir()
	if err := os.Symlink("/dev/full", filepath.Join(d, "events.jsonl")); err != nil {
		t.Skip(err)
	}
	if err := (Store{Dir: d}).Append([]world.Event{{Turn: 1, Type: "lost"}}); err == nil {
		t.Fatal("Append dropped a write error (bufio flush error ignored by defer)")
	}
}
