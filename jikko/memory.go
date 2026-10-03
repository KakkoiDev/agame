package jikko

import (
	"encoding/json"
	"os"
	"sort"
	"strings"
	"sync"
)

// Identity is a ruler, or a group (an alliance) when Group is set.
type Identity struct {
	ID      string   `json:"id"`
	Name    string   `json:"name,omitempty"`
	Group   bool     `json:"group,omitempty"`
	Members []string `json:"members,omitempty"`
}

// Version is one committed state of a document: the history Git would keep.
type Version struct {
	// Rev is the store revision that committed it.
	Rev     int    `json:"rev"`
	Turn    int    `json:"turn"`
	Author  string `json:"author"`
	Op      string `json:"op"`
	Content string `json:"content"`
}

// record is a stored document with its access list and full history.
type record struct {
	Path      string    `json:"path"`
	Type      string    `json:"type"`
	Owner     string    `json:"owner"`
	ReadOnly  bool      `json:"read_only,omitempty"`
	Retracted bool      `json:"retracted,omitempty"`
	Versions  []Version `json:"versions"`
}

func (r *record) doc() Doc {
	v := r.Versions[len(r.Versions)-1]
	return Doc{Path: r.Path, Title: Title(r.Path, v.Content), Type: r.Type, Revision: len(r.Versions), Content: v.Content, Retracted: r.Retracted, ReadOnly: r.ReadOnly}
}

// AuditEntry is one committed change, in commit order.
type AuditEntry struct {
	Rev    int    `json:"rev"`
	Turn   int    `json:"turn"`
	Author string `json:"author"`
	Op     string `json:"op"`
	Path   string `json:"path,omitempty"`
}

// Memory is an in-process Store. Its JSON form (maps with sorted keys) is
// deterministic, so a run that saves it after every turn stays reproducible.
type Memory struct {
	mu         sync.Mutex
	Rev        int                  `json:"revision"`
	Identities map[string]*Identity `json:"identities"`
	Docs       map[string]*record   `json:"docs"`
	Audit      []AuditEntry         `json:"audit"`
}

// NewMemory returns an empty store at revision 0.
func NewMemory() *Memory {
	return &Memory{Identities: map[string]*Identity{}, Docs: map[string]*record{}}
}

// LoadMemory reads a store saved with Save; a missing file is an empty store.
func LoadMemory(path string) (*Memory, error) {
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return NewMemory(), nil
	}
	if err != nil {
		return nil, err
	}
	m := NewMemory()
	if err := json.Unmarshal(b, m); err != nil {
		return nil, err
	}
	if m.Identities == nil {
		m.Identities = map[string]*Identity{}
	}
	if m.Docs == nil {
		m.Docs = map[string]*record{}
	}
	return m, nil
}

// MarshalJSON locks the store while encoding it.
func (m *Memory) MarshalJSON() ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	type plain struct {
		Rev        int                  `json:"revision"`
		Identities map[string]*Identity `json:"identities"`
		Docs       map[string]*record   `json:"docs"`
		Audit      []AuditEntry         `json:"audit"`
	}
	return json.Marshal(plain{m.Rev, m.Identities, m.Docs, m.Audit})
}

func (m *Memory) Revision() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.Rev
}

func (m *Memory) Groups(who string) []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.groups(who)
}

func (m *Memory) groups(who string) []string {
	var out []string
	for id, x := range m.Identities {
		if x.Group {
			for _, mem := range x.Members {
				if mem == who {
					out = append(out, id)
				}
			}
		}
	}
	sort.Strings(out)
	return out
}

// readable: the owner principal is who or a group who belongs to.
func (m *Memory) readable(who string, r *record) bool {
	if r.Owner == who {
		return true
	}
	if g := m.Identities[r.Owner]; g != nil && g.Group {
		for _, mem := range g.Members {
			if mem == who {
				return true
			}
		}
	}
	return false
}

func (m *Memory) Tree(who string) []Entry {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []Entry
	for _, p := range sortedKeys(m.Docs) {
		r := m.Docs[p]
		if m.readable(who, r) {
			d := r.doc()
			out = append(out, Entry{Path: d.Path, Title: d.Title, Type: d.Type, Revision: d.Revision, Retracted: d.Retracted, ReadOnly: d.ReadOnly})
		}
	}
	return out
}

func (m *Memory) Read(who, path string) (Doc, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r := m.Docs[path]
	if r == nil || !m.readable(who, r) {
		return Doc{}, false
	}
	return r.doc(), true
}

// Apply commits the writes as one transaction: the store revision advances
// once if any write succeeded.
func (m *Memory) Apply(who string, turn int, writes []Write) []WriteResult {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]WriteResult, len(writes))
	rev := m.Rev + 1
	changed := false
	groups := m.groups(who)
	for i, w := range writes {
		out[i] = WriteResult{Op: w.Op, Path: w.Path}
		r := m.Docs[w.Path]
		var cur *Doc
		if r != nil && m.readable(who, r) {
			d := r.doc()
			cur = &d
		}
		if err := m.permit(who, groups, w, r); err != nil {
			out[i].Error = err.Error()
			continue
		}
		next, err := Next(cur, w, who, turn)
		if err != nil {
			out[i].Error = err.Error()
			continue
		}
		if r == nil {
			r = &record{Path: w.Path, Type: next.Type, Owner: ownerOf(w.Path)}
			m.Docs[w.Path] = r
		}
		r.Retracted = next.Retracted
		r.Versions = append(r.Versions, Version{Rev: rev, Turn: turn, Author: who, Op: w.Op, Content: next.Content})
		m.Audit = append(m.Audit, AuditEntry{Rev: rev, Turn: turn, Author: who, Op: w.Op, Path: w.Path})
		out[i].Revision = next.Revision
		changed = true
	}
	if changed {
		m.Rev = rev
	}
	return out
}

// permit checks who may make write w. Refusals never reveal whether an
// unreadable document exists.
func (m *Memory) permit(who string, groups []string, w Write, r *record) error {
	if r != nil && !m.readable(who, r) {
		r = nil
	}
	if r != nil && r.ReadOnly {
		return errReadOnly(w.Path)
	}
	for _, ws := range Workspaces(who, groups) {
		if strings.HasPrefix(w.Path, ws) {
			return nil
		}
	}
	return errWorkspace(who, groups)
}

// SyncGroups sets every group's members. The revision advances only when
// membership actually changed, since membership changes what is readable.
func (m *Memory) SyncGroups(groups map[string][]string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	changed := false
	for _, id := range sortedKeys(groups) {
		mem := sortedSet(groups[id])
		x := m.Identities[id]
		if x == nil {
			x = &Identity{ID: id, Group: true}
			m.Identities[id] = x
		}
		if !equal(x.Members, mem) {
			x.Members, changed = mem, true
		}
	}
	for _, id := range sortedKeys(m.Identities) {
		if x := m.Identities[id]; x.Group && groups[id] == nil && len(x.Members) > 0 {
			x.Members, changed = nil, true
		}
	}
	if changed {
		m.Rev++
		m.Audit = append(m.Audit, AuditEntry{Rev: m.Rev, Author: Harness, Op: "groups"})
	}
}

func (m *Memory) EnsureRuler(id, name, persona string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Identities[id] != nil {
		return
	}
	m.Rev++
	m.Identities[id] = &Identity{ID: id, Name: name}
	p := "rulers/" + id + "/identity.md"
	m.Docs[p] = &record{Path: p, Type: TypeDocument, Owner: id, ReadOnly: true,
		Versions: []Version{{Rev: m.Rev, Author: Harness, Op: OpCreate, Content: persona}}}
	m.Audit = append(m.Audit, AuditEntry{Rev: m.Rev, Author: Harness, Op: "identity", Path: p})
}

// History returns every version of a document who may read, oldest first:
// the auditable prior text that corrections and retractions never erase.
func (m *Memory) History(who, path string) []Version {
	m.mu.Lock()
	defer m.mu.Unlock()
	r := m.Docs[path]
	if r == nil || !m.readable(who, r) {
		return nil
	}
	return append([]Version(nil), r.Versions...)
}

func errReadOnly(p string) error {
	return &permError{"document " + p + " is read-only"}
}

func errWorkspace(who string, groups []string) error {
	return &permError{"you may only write under " + strings.Join(Workspaces(who, groups), " or ")}
}

type permError struct{ s string }

func (e *permError) Error() string { return e.s }

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
