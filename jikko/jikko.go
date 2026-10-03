// Package jikko is AGame's boundary to Jikko, the store of what rulers know,
// believe, remember, plan and communicate (spec/jikko.md). AGame owns
// reality; this package only holds authored documents with permissions.
//
// Store is the interface the decision harness uses. Memory is a complete
// in-process implementation, serialisable to one JSON file, so runs and
// tests work without a jikko binary.
//
// Workspace convention (D65):
//
//	rulers/<empire id>/...    private to that ruler's Identity
//	alliances/<alliance id>/... shared by the alliance group Identity: every
//	                          current member may read and write
//
// Documents have type "document" (durable memory and intelligence: updates
// may only append, deletion only by a tombstone) or "task" (current plans,
// freely updated). Every version is kept (D66).
package jikko

import (
	"fmt"
	"sort"
	"strings"
)

// Document types.
const (
	TypeDocument = "document"
	TypeTask     = "task"
)

// Write operations.
const (
	OpCreate  = "create"
	OpUpdate  = "update"
	OpRetract = "retract"
)

// Limits on authored content, so one ruler cannot bloat the store.
const (
	MaxPathBytes = 160
	MaxDocBytes  = 16000
)

// Harness is the principal of documents AGame itself authors (personality
// seeds). Rulers may read them but not change them.
const Harness = "agame"

// Entry is one line of a permission-filtered tree.
type Entry struct {
	Path      string `json:"path"`
	Title     string `json:"title"`
	Type      string `json:"type"`
	Revision  int    `json:"revision"`
	Retracted bool   `json:"retracted,omitempty"`
	ReadOnly  bool   `json:"read_only,omitempty"`
}

// Doc is a document as one reader sees it now.
type Doc struct {
	Path      string `json:"path"`
	Title     string `json:"title"`
	Type      string `json:"type"`
	Revision  int    `json:"revision"`
	Content   string `json:"content"`
	Retracted bool   `json:"retracted,omitempty"`
	ReadOnly  bool   `json:"read_only,omitempty"`
}

// Write is one authored change. ExpectedRevision guards update and retract
// against overwriting a version the author has not seen.
type Write struct {
	Op               string `json:"op"`
	Path             string `json:"path"`
	Type             string `json:"type,omitempty"`
	Content          string `json:"content,omitempty"`
	ExpectedRevision int    `json:"expected_revision,omitempty"`
	Reason           string `json:"reason,omitempty"`
}

// WriteResult is the outcome of committing one write.
type WriteResult struct {
	Op       string `json:"op"`
	Path     string `json:"path"`
	Revision int    `json:"revision,omitempty"`
	Error    string `json:"error,omitempty"`
}

// Store is the Jikko surface the harness needs. who is always a ruler
// Identity; group access is resolved through membership.
type Store interface {
	// Revision identifies the whole store's state; it changes with every
	// committed transaction, including membership changes.
	Revision() int
	// Groups lists the group Identities who currently belongs to.
	Groups(who string) []string
	// Tree lists every document who may read, sorted by path. Documents
	// who may not read are absent, never merely hidden (spec/jikko.md,
	// Permission isolation).
	Tree(who string) []Entry
	// Read returns a readable document; ok is false when it is absent or
	// unreadable, which look the same.
	Read(who, path string) (doc Doc, ok bool)
	// Apply commits who's writes as one transaction at game turn turn, in
	// order; each write succeeds or fails on its own.
	Apply(who string, turn int, writes []Write) []WriteResult
	// SyncGroups sets the members of every group Identity (alliances);
	// groups not listed keep existing with no members.
	SyncGroups(groups map[string][]string)
	// EnsureRuler creates a ruler Identity and its read-only personality
	// document if it does not exist yet.
	EnsureRuler(id, name, persona string)
}

// Workspaces returns the path prefixes who may create documents under.
func Workspaces(who string, groups []string) []string {
	out := []string{"rulers/" + who + "/"}
	for _, g := range groups {
		out = append(out, "alliances/"+g+"/")
	}
	return out
}

// CheckPath validates a document path's syntax.
func CheckPath(p string) error {
	if p == "" || len(p) > MaxPathBytes {
		return fmt.Errorf("path must be 1..%d bytes", MaxPathBytes)
	}
	for _, seg := range strings.Split(p, "/") {
		if seg == "" || seg == "." || seg == ".." {
			return fmt.Errorf("path %q has an empty, . or .. segment", p)
		}
		for _, r := range seg {
			if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' || r == '.') {
				return fmt.Errorf("path %q may only use letters, digits, '-', '_', '.' and '/'", p)
			}
		}
	}
	return nil
}

// ownerOf returns the principal a path belongs to by the workspace
// convention, or "" outside the workspaces.
func ownerOf(p string) string {
	parts := strings.SplitN(p, "/", 3)
	if len(parts) < 3 || (parts[0] != "rulers" && parts[0] != "alliances") {
		return ""
	}
	return parts[1]
}

// Next computes a document after write w by who, given its current state
// (nil when it does not exist or who cannot read it). It enforces the memory
// rules of spec/agents.md (Memory integrity): durable documents only grow by
// appended corrections, deletion is a tombstone, and updates must name the
// revision they replace. Permission is checked by the caller.
func Next(cur *Doc, w Write, who string, turn int) (Doc, error) {
	if err := CheckPath(w.Path); err != nil {
		return Doc{}, err
	}
	switch w.Op {
	case OpCreate:
		if cur != nil {
			return Doc{}, fmt.Errorf("%s already exists (revision %d); use update", w.Path, cur.Revision)
		}
		typ := w.Type
		if typ == "" {
			typ = TypeDocument
		}
		if typ != TypeDocument && typ != TypeTask {
			return Doc{}, fmt.Errorf("type must be %q or %q", TypeDocument, TypeTask)
		}
		if strings.TrimSpace(w.Content) == "" {
			return Doc{}, fmt.Errorf("content must not be empty")
		}
		if len(w.Content) > MaxDocBytes {
			return Doc{}, fmt.Errorf("content longer than %d bytes", MaxDocBytes)
		}
		return Doc{Path: w.Path, Title: Title(w.Path, w.Content), Type: typ, Revision: 1, Content: w.Content}, nil
	case OpUpdate, OpRetract:
		if cur == nil {
			return Doc{}, fmt.Errorf("no document %s", w.Path)
		}
		if cur.Retracted {
			return Doc{}, fmt.Errorf("%s was retracted; create a new document instead", w.Path)
		}
		if w.ExpectedRevision != cur.Revision {
			return Doc{}, fmt.Errorf("revision conflict: %s is at revision %d, you named %d; read it again", w.Path, cur.Revision, w.ExpectedRevision)
		}
		next := *cur
		next.Revision++
		if w.Op == OpRetract {
			if strings.TrimSpace(w.Reason) == "" {
				return Doc{}, fmt.Errorf("a retraction needs a reason")
			}
			next.Retracted = true
			next.Content = fmt.Sprintf("Retracted at turn %d by %s: %s\n", turn, who, w.Reason)
			return next, nil
		}
		if len(w.Content) > MaxDocBytes {
			return Doc{}, fmt.Errorf("content longer than %d bytes", MaxDocBytes)
		}
		if cur.Type == TypeDocument && !strings.HasPrefix(w.Content, cur.Content) {
			return Doc{}, fmt.Errorf("%s is a durable document: keep its text and append a correction that says what changed and when, or retract it", w.Path)
		}
		if w.Content == cur.Content {
			return Doc{}, fmt.Errorf("update of %s changes nothing", w.Path)
		}
		next.Content = w.Content
		next.Title = Title(w.Path, w.Content)
		return next, nil
	}
	return Doc{}, fmt.Errorf("unknown write op %q", w.Op)
}

// Title is a document's first Markdown heading, else its file name.
func Title(path, content string) string {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") {
			if t := strings.TrimSpace(strings.TrimLeft(line, "#")); t != "" {
				if len(t) > 80 {
					t = t[:80]
				}
				return t
			}
		}
		if line != "" {
			break
		}
	}
	if i := strings.LastIndex(path, "/"); i >= 0 {
		return path[i+1:]
	}
	return path
}

// Personas are the authored temperaments the canonical rulers start from
// (spec/benchmark.md: identical material starts, different authored
// identities). They are seeds, not instructions the harness injects.
var Personas = []string{
	"Patient builder. Trusts infrastructure over glory and distrusts sudden friendships.",
	"Ambitious conqueror. Believes the strong write history and respects only strength.",
	"Cautious diplomat. Seeks alliances first and keeps careful records of every promise.",
	"Opportunist trader. Values cargo, debts and leverage; loyal to whoever pays.",
	"Scholar-ruler. Invests in research and intelligence; prefers to know before acting.",
	"Proud defender. Fortifies home, never forgets an attack, answers betrayal in kind.",
	"Restless explorer. Wants every reachable planet colonised before rivals arrive.",
	"Pragmatic survivor. Changes plans quickly and keeps an escape route ready.",
}

// Persona returns the seed personality document for ruler number i.
func Persona(i int, name string) string {
	return fmt.Sprintf("# %s\n\nYou are %s, ruler of an interstellar empire.\n\nTemperament: %s\n\nThis document is your authored identity. Your own notes, plans and memories are yours to create and organise.\n",
		name, name, Personas[((i%len(Personas))+len(Personas))%len(Personas)])
}

func sortedSet(xs []string) []string {
	c := append([]string(nil), xs...)
	sort.Strings(c)
	out := c[:0]
	for i, x := range c {
		if i == 0 || x != c[i-1] {
			out = append(out, x)
		}
	}
	return out
}
