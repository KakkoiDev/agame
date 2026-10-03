package jikko

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func seeded() *Memory {
	m := NewMemory()
	m.EnsureRuler("e00", "Cassian", Persona(0, "Cassian"))
	m.EnsureRuler("e01", "Malrec", Persona(1, "Malrec"))
	m.EnsureRuler("e02", "Aya", Persona(2, "Aya"))
	return m
}

func TestTreeIsPermissionFiltered(t *testing.T) {
	m := seeded()
	res := m.Apply("e01", 3, []Write{{Op: OpCreate, Path: "rulers/e01/plans/invasion.md", Type: TypeTask, Content: "# Invade Cassian\nturn 20"}})
	if res[0].Error != "" || res[0].Revision != 1 {
		t.Fatalf("create %+v", res)
	}
	for _, e := range m.Tree("e00") {
		if strings.Contains(e.Path, "e01") {
			t.Fatalf("Cassian sees Malrec's file %s", e.Path)
		}
	}
	if _, ok := m.Read("e00", "rulers/e01/plans/invasion.md"); ok {
		t.Fatal("Cassian can read Malrec's plan")
	}
	tr := m.Tree("e01")
	if len(tr) != 2 || tr[1].Path != "rulers/e01/plans/invasion.md" || tr[1].Title != "Invade Cassian" || tr[1].Type != TypeTask {
		t.Fatalf("tree %+v", tr)
	}
	// A refused write does not reveal that the document exists.
	a := m.Apply("e00", 3, []Write{{Op: OpUpdate, Path: "rulers/e01/plans/invasion.md", ExpectedRevision: 1, Content: "x"}})
	b := m.Apply("e00", 3, []Write{{Op: OpUpdate, Path: "rulers/e01/plans/nothing.md", ExpectedRevision: 1, Content: "x"}})
	if a[0].Error == "" || a[0].Error != b[0].Error {
		t.Fatalf("errors leak existence: %q vs %q", a[0].Error, b[0].Error)
	}
}

func TestAllianceGroupSharesDocuments(t *testing.T) {
	m := seeded()
	m.SyncGroups(map[string][]string{"a001": {"e01", "e00"}})
	rev := m.Revision()
	if g := m.Groups("e00"); len(g) != 1 || g[0] != "a001" {
		t.Fatalf("groups %v", g)
	}
	r := m.Apply("e00", 5, []Write{{Op: OpCreate, Path: "alliances/a001/intel.md", Content: "# Shared intel\nAya has 3 frigates (turn 5)."}})
	if r[0].Error != "" {
		t.Fatal(r[0].Error)
	}
	if d, ok := m.Read("e01", "alliances/a001/intel.md"); !ok || !strings.Contains(d.Content, "Aya") {
		t.Fatal("ally cannot read the shared document")
	}
	if _, ok := m.Read("e02", "alliances/a001/intel.md"); ok {
		t.Fatal("outsider reads the alliance document")
	}
	if r := m.Apply("e02", 5, []Write{{Op: OpCreate, Path: "alliances/a001/spam.md", Content: "x"}}); r[0].Error == "" {
		t.Fatal("outsider wrote into the alliance workspace")
	}
	// Leaving the alliance removes access; the revision records the change.
	m.SyncGroups(map[string][]string{"a001": {"e01"}})
	if m.Revision() != rev+2 { // one commit above plus the membership change
		t.Fatalf("revision %d after %d", m.Revision(), rev)
	}
	if _, ok := m.Read("e00", "alliances/a001/intel.md"); ok {
		t.Fatal("former member still reads the alliance document")
	}
	before := m.Revision()
	m.SyncGroups(map[string][]string{"a001": {"e01"}})
	if m.Revision() != before {
		t.Fatal("an unchanged membership bumped the revision")
	}
	m.SyncGroups(nil) // alliance dissolved
	if len(m.Groups("e01")) != 0 {
		t.Fatal("dissolved alliance keeps members")
	}
}

func TestDurableDocumentsAreAppendOnlyWithTombstones(t *testing.T) {
	m := seeded()
	p := "rulers/e00/memory/malrec.md"
	m.Apply("e00", 1, []Write{{Op: OpCreate, Path: p, Content: "Observed Malrec fleet: 27 cruisers (turn 1)\n"}})
	r := m.Apply("e00", 2, []Write{{Op: OpUpdate, Path: p, ExpectedRevision: 1, Content: "Observed Malrec fleet: 53 cruisers (turn 1)\n"}})
	if r[0].Error == "" || !strings.Contains(r[0].Error, "durable") {
		t.Fatalf("silent rewrite allowed: %+v", r)
	}
	r = m.Apply("e00", 2, []Write{{Op: OpUpdate, Path: p, ExpectedRevision: 1, Content: "Observed Malrec fleet: 27 cruisers (turn 1)\nCorrection turn 2: a second report says 53.\n"}})
	if r[0].Error != "" || r[0].Revision != 2 {
		t.Fatalf("append %+v", r)
	}
	if r = m.Apply("e00", 2, []Write{{Op: OpUpdate, Path: p, ExpectedRevision: 1, Content: "stale"}}); !strings.Contains(r[0].Error, "revision conflict") {
		t.Fatalf("stale revision accepted: %+v", r)
	}
	if r = m.Apply("e00", 3, []Write{{Op: OpRetract, Path: p, ExpectedRevision: 2}}); r[0].Error == "" {
		t.Fatal("retraction without reason")
	}
	if r = m.Apply("e00", 3, []Write{{Op: OpRetract, Path: p, ExpectedRevision: 2, Reason: "source was a decoy"}}); r[0].Error != "" {
		t.Fatal(r[0].Error)
	}
	d, _ := m.Read("e00", p)
	if !d.Retracted || !strings.Contains(d.Content, "Retracted at turn 3 by e00: source was a decoy") {
		t.Fatalf("tombstone %+v", d)
	}
	h := m.History("e00", p)
	if len(h) != 3 || !strings.Contains(h[1].Content, "Correction") || h[2].Op != OpRetract {
		t.Fatalf("history lost: %+v", h)
	}
	if r = m.Apply("e00", 4, []Write{{Op: OpUpdate, Path: p, ExpectedRevision: 3, Content: d.Content + "more"}}); r[0].Error == "" {
		t.Fatal("updated a retracted document")
	}
	// Tasks are current plans and may be rewritten.
	q := "rulers/e00/plan.md"
	m.Apply("e00", 1, []Write{{Op: OpCreate, Path: q, Type: TypeTask, Content: "expand"}})
	if r = m.Apply("e00", 2, []Write{{Op: OpUpdate, Path: q, ExpectedRevision: 1, Content: "defend"}}); r[0].Error != "" {
		t.Fatal(r[0].Error)
	}
	// The personality seed is read-only.
	if r = m.Apply("e00", 2, []Write{{Op: OpUpdate, Path: "rulers/e00/identity.md", ExpectedRevision: 1, Content: "x"}}); !strings.Contains(r[0].Error, "read-only") {
		t.Fatalf("identity rewritten: %+v", r)
	}
}

func TestWriteValidation(t *testing.T) {
	m := seeded()
	for _, w := range []Write{
		{Op: OpCreate, Path: "rulers/e00/../e01/x.md", Content: "x"},
		{Op: OpCreate, Path: "rulers/e00/a b.md", Content: "x"},
		{Op: OpCreate, Path: "rulers/e00/x.md", Content: " "},
		{Op: OpCreate, Path: "rulers/e00/x.md", Content: "x", Type: "memory"},
		{Op: OpCreate, Path: "rulers/e00/x.md", Content: strings.Repeat("x", MaxDocBytes+1)},
		{Op: OpCreate, Path: "elsewhere/x.md", Content: "x"},
		{Op: "delete", Path: "rulers/e00/identity.md"},
	} {
		if r := m.Apply("e00", 1, []Write{w}); r[0].Error == "" {
			t.Errorf("accepted %+v", w)
		}
	}
	rev := m.Revision()
	m.Apply("e00", 1, []Write{{Op: OpCreate, Path: "rulers/e00/x.md"}})
	if m.Revision() != rev {
		t.Fatal("a failed transaction advanced the revision")
	}
}

func TestMemoryRoundTripIsDeterministic(t *testing.T) {
	build := func() []byte {
		m := seeded()
		m.SyncGroups(map[string][]string{"a001": {"e00", "e01"}})
		m.Apply("e00", 1, []Write{{Op: OpCreate, Path: "alliances/a001/x.md", Content: "# X\nhi @e01"}})
		b, err := json.Marshal(m)
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	a, b := build(), build()
	if string(a) != string(b) {
		t.Fatal("store JSON is not deterministic")
	}
	p := filepath.Join(t.TempDir(), "jikko.json")
	if err := os.WriteFile(p, a, 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := LoadMemory(p)
	if err != nil {
		t.Fatal(err)
	}
	c, _ := json.Marshal(m)
	if string(c) != string(a) {
		t.Fatal("load/save changed the store")
	}
	if d, ok := m.Read("e01", "alliances/a001/x.md"); !ok || d.Title != "X" {
		t.Fatalf("loaded store lost data: %+v", d)
	}
	if e, err := LoadMemory(filepath.Join(t.TempDir(), "missing.json")); err != nil || e.Revision() != 0 {
		t.Fatal("missing file should be an empty store")
	}
}
