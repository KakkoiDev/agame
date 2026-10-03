package run

import (
	"strings"
	"testing"

	"github.com/KakkoiDev/agame/world"
)

func TestRoundTrip(t *testing.T) {
	d := t.TempDir()
	s := Store{Dir: d}
	w, _ := world.Generate(9, []string{"A", "B", "C", "D", "E", "F", "G", "H"})
	if err := s.Save(w); err != nil {
		t.Fatal(err)
	}
	x, err := s.Load()
	if err != nil || x.Seed != 9 || len(x.Planets) != 128 {
		t.Fatal("roundtrip")
	}
}

func TestTailAndCommit(t *testing.T) {
	s := Store{Dir: t.TempDir()}
	if l, err := s.Tail("missing.jsonl", 3); err != nil || l != nil {
		t.Fatalf("%v %v", l, err)
	}
	big := strings.Repeat("x", 70<<10) // forces several backwards reads
	for i := 0; i < 5; i++ {
		if err := s.AppendJSONL("log.jsonl", map[string]any{"i": i, "pad": big}); err != nil {
			t.Fatal(err)
		}
	}
	lines, err := s.Tail("log.jsonl", 2)
	if err != nil || len(lines) != 2 || !strings.HasPrefix(string(lines[0]), `{"i":3`) || !strings.HasPrefix(string(lines[1]), `{"i":4`) {
		t.Fatalf("tail %d %v", len(lines), err)
	}
	if lines, _ := s.Tail("log.jsonl", 50); len(lines) != 5 {
		t.Fatalf("tail all %d", len(lines))
	}
	w, _ := world.Generate(1, []string{"A", "B", "C", "D", "E", "F", "G", "H"})
	if err := s.Create(w, Header{}); err != nil {
		t.Fatal(err)
	}
	if err := s.Create(w, Header{}); err == nil {
		t.Fatal("Create overwrote a universe")
	}
	res, _ := world.ResolveTurn(w, nil)
	if err := s.Commit(w, res, map[string]int{"d": 1}); err != nil {
		t.Fatal(err)
	}
	turns, err := s.Turns()
	if err != nil || len(turns) != 1 || turns[0].StateHash != res.StateHash {
		t.Fatalf("turns %v %v", turns, err)
	}
	var h Header
	if err := s.LoadJSON(HeaderFile, &h); err != nil || h.Ruleset != world.RulesetVersion || h.InitialHash == "" {
		t.Fatalf("header %+v", h)
	}
	if err := s.AppendJSONL(TurnsFile, "not a turn"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Turns(); err == nil {
		t.Fatal("corrupt turn log accepted")
	}
}
