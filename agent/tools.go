package agent

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/KakkoiDev/agame/jikko"
	"github.com/KakkoiDev/agame/world"
)

// ToolBudget holds the per-ruler, per-turn tool and context limits of
// spec/agents.md (Small-model budgets; D27). They are benchmark settings and
// are recorded with the run.
type ToolBudget struct {
	Rounds        int `json:"rounds"`
	ReadMany      int `json:"read_many"`
	Files         int `json:"files"`
	InputTokens   int `json:"input_tokens"`
	InitialTokens int `json:"initial_tokens"`
	OutputTokens  int `json:"output_tokens"`
}

// CanonicalToolBudget is the canonical 2B-8B budget: 8 rounds, read_many of
// up to 16 files, 24 files returned, 24k input tokens, an 8k initial context
// target and a 2k decision output.
func CanonicalToolBudget() ToolBudget {
	return ToolBudget{Rounds: 8, ReadMany: 16, Files: 24, InputTokens: 24000, InitialTokens: 8000, OutputTokens: DefaultMaxTokens}
}

// ReflectionRounds is the separate tool-round budget of a reflection phase.
const ReflectionRounds = 4

// Limits of the tree listing and of search results.
const (
	TreeLimit    = 48
	SearchLimit  = 16
	snippetBytes = 160
)

// ApproxTokens is the deterministic token estimate used for every budget:
// one token per four bytes, rounded up (D67). Real tokenizers differ; the
// estimate only has to be the same on every machine.
func ApproxTokens(s string) int { return (len(s) + 3) / 4 }

// Tool names (spec/agents.md, Tool surface).
const (
	ToolTree     = "jikko.tree"
	ToolRead     = "jikko.read"
	ToolReadMany = "jikko.read_many"
	ToolSearch   = "jikko.search"
	ToolCreate   = "jikko.create"
	ToolUpdate   = "jikko.update"
	ToolRetract  = "jikko.retract"
	ToolMentions = "jikko.mentions"
	ToolInspect  = "game.inspect"
	ToolSubmit   = "game.submit"
)

// ToolNames lists every tool in a fixed order.
var ToolNames = []string{ToolTree, ToolRead, ToolReadMany, ToolSearch, ToolCreate, ToolUpdate, ToolRetract, ToolMentions, ToolInspect, ToolSubmit}

// ToolCall is one tool invocation in the JSON action protocol.
type ToolCall struct {
	Tool             string        `json:"tool"`
	Path             string        `json:"path,omitempty"`
	Paths            []string      `json:"paths,omitempty"`
	Query            string        `json:"query,omitempty"`
	Content          string        `json:"content,omitempty"`
	Type             string        `json:"type,omitempty"`
	ExpectedRevision int           `json:"expected_revision,omitempty"`
	Reason           string        `json:"reason,omitempty"`
	Ref              string        `json:"ref,omitempty"`
	Orders           []world.Order `json:"orders,omitempty"`
	Statement        string        `json:"statement,omitempty"`
	Note             string        `json:"note,omitempty"`
}

// FileRef is a document revision a ruler retrieved.
type FileRef struct {
	Path     string `json:"path"`
	Revision int    `json:"revision"`
}

// ToolCallRecord is the audit line of one tool call.
type ToolCallRecord struct {
	Round int    `json:"round"`
	Tool  string `json:"tool"`
	Arg   string `json:"arg,omitempty"`
	Error string `json:"error,omitempty"`
}

// ToolUsage is what a ruler did with its tools in one decision or
// reflection: the answer to "what could this ruler know when it decided?"
// (spec/jikko.md, Audit).
type ToolUsage struct {
	Identity      string              `json:"identity"`
	Groups        []string            `json:"groups,omitempty"`
	TreeRevision  int                 `json:"tree_revision"`
	TreeSize      int                 `json:"tree_size"`
	Rounds        int                 `json:"rounds"`
	Calls         []ToolCallRecord    `json:"calls,omitempty"`
	FilesRead     []FileRef           `json:"files_read,omitempty"`
	FilesReturned int                 `json:"files_returned"`
	Writes        []jikko.WriteResult `json:"writes,omitempty"`
	InputTokens   int                 `json:"input_tokens"`
	OutputTokens  int                 `json:"output_tokens"`
	// Exhausted names the first budget the ruler ran into.
	Exhausted string `json:"budget_exhausted,omitempty"`
}

// Tools is one ruler's tool session for one decision (or reflection). It
// reads the store as committed before the turn; writes are staged, visible
// to this ruler at once and committed only after the decision barrier, so no
// ruler can see another's same-turn writes and invocation order keeps no
// game meaning (D68). Tools is safe for concurrent use.
type Tools struct {
	mu         sync.Mutex
	store      jikko.Store
	obs        Observation
	budget     ToolBudget
	reflection bool
	closed     bool
	overlay    map[string]*jikko.Doc
	staged     []jikko.Write
	usage      ToolUsage
}

// NewTools opens a session for o's empire on store. reflection selects the
// reflection budget (ReflectionRounds) and forbids game.submit.
func NewTools(store jikko.Store, o Observation, b ToolBudget, reflection bool) *Tools {
	if b == (ToolBudget{}) {
		b = CanonicalToolBudget()
	}
	if reflection {
		b.Rounds = ReflectionRounds
	}
	who := ""
	if o.Empire != nil {
		who = o.Empire.ID
	}
	t := &Tools{store: store, obs: o, budget: b, reflection: reflection, overlay: map[string]*jikko.Doc{}}
	t.usage = ToolUsage{Identity: who, Groups: store.Groups(who), TreeRevision: store.Revision()}
	t.usage.TreeSize = len(store.Tree(who))
	return t
}

// Budget returns the session's limits.
func (t *Tools) Budget() ToolBudget { return t.budget }

// Reflection reports whether this is a reflection session.
func (t *Tools) Reflection() bool { return t.reflection }

// Identity is the authenticated Jikko identity.
func (t *Tools) Identity() string { return t.usage.Identity }

// Close ends the session: later calls fail and the staged writes are final.
func (t *Tools) Close() {
	t.mu.Lock()
	t.closed = true
	t.mu.Unlock()
}

// Staged returns the writes to commit after the barrier.
func (t *Tools) Staged() []jikko.Write {
	t.mu.Lock()
	defer t.mu.Unlock()
	return append([]jikko.Write(nil), t.staged...)
}

// Usage returns a copy of the audit record.
func (t *Tools) Usage() ToolUsage {
	t.mu.Lock()
	defer t.mu.Unlock()
	u := t.usage
	u.Groups = append([]string(nil), u.Groups...)
	u.Calls = append([]ToolCallRecord(nil), u.Calls...)
	u.FilesRead = append([]FileRef(nil), u.FilesRead...)
	u.Writes = append([]jikko.WriteResult(nil), u.Writes...)
	return u
}

// CountInput adds harness-supplied text (prompts, repair requests) to the
// input-token count and reports whether it still fits the budget.
func (t *Tools) CountInput(s string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.usage.InputTokens += ApproxTokens(s)
	if t.usage.InputTokens > t.budget.InputTokens {
		t.exhaust("input_tokens")
		return false
	}
	return true
}

// CountOutput adds model output to the output-token count.
func (t *Tools) CountOutput(s string) {
	t.mu.Lock()
	t.usage.OutputTokens += ApproxTokens(s)
	t.mu.Unlock()
}

// Exhausted reports whether the session can take no more tool rounds.
func (t *Tools) Exhausted() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.closed || t.usage.Rounds >= t.budget.Rounds || t.usage.InputTokens >= t.budget.InputTokens
}

func (t *Tools) exhaust(what string) {
	if t.usage.Exhausted == "" {
		t.usage.Exhausted = what
	}
}

// Context is the Jikko part of the initial invocation context: the
// authenticated identity, writable workspaces, the permission-filtered tree
// (bounded, with directory counts when large) and the budget. It does not
// include any document's content (spec/agents.md, Initial invocation
// context).
func (t *Tools) Context() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	var b strings.Builder
	u := t.usage
	fmt.Fprintf(&b, "Jikko identity: %s", u.Identity)
	if len(u.Groups) > 0 {
		fmt.Fprintf(&b, " (member of %s)", strings.Join(u.Groups, ", "))
	}
	fmt.Fprintf(&b, "\nYou may write under: %s\n", strings.Join(jikko.Workspaces(u.Identity, u.Groups), " "))
	fmt.Fprintf(&b, "Accessible Jikko tree at revision %d (%d documents):\n", u.TreeRevision, u.TreeSize)
	for _, line := range boundedTree(t.tree(), "", TreeLimit) {
		b.WriteString(line)
		b.WriteByte('\n')
	}
	bud := t.budget
	if t.reflection {
		fmt.Fprintf(&b, "Reflection budget: %d tool rounds; you cannot give orders now.\n", bud.Rounds)
	} else {
		fmt.Fprintf(&b, "Budget this turn: %d tool rounds, read_many <= %d paths, %d files returned, %d input tokens, %d output tokens per reply.\n",
			bud.Rounds, bud.ReadMany, bud.Files, bud.InputTokens, bud.OutputTokens)
	}
	return b.String()
}

// tree is the store tree plus this session's staged documents.
func (t *Tools) tree() []jikko.Entry {
	es := t.store.Tree(t.usage.Identity)
	seen := map[string]int{}
	for i, e := range es {
		seen[e.Path] = i
	}
	for _, p := range sortedIDs(t.overlay) {
		d := t.overlay[p]
		e := jikko.Entry{Path: d.Path, Title: d.Title, Type: d.Type, Revision: d.Revision, Retracted: d.Retracted, ReadOnly: d.ReadOnly}
		if i, ok := seen[p]; ok {
			es[i] = e
		} else {
			es = append(es, e)
		}
	}
	sort.Slice(es, func(i, j int) bool { return es[i].Path < es[j].Path })
	return es
}

func (t *Tools) read(p string) (jikko.Doc, bool) {
	if d := t.overlay[p]; d != nil {
		return *d, true
	}
	return t.store.Read(t.usage.Identity, p)
}

// boundedTree renders entries under prefix, one per line. When there are
// more than limit, deeper levels collapse into "dir/ (N documents)" lines at
// the deepest level that fits; the ruler can list a directory with
// jikko.tree {"path": "dir/"}.
func boundedTree(es []jikko.Entry, prefix string, limit int) []string {
	var in []jikko.Entry
	for _, e := range es {
		if strings.HasPrefix(e.Path, prefix) {
			in = append(in, e)
		}
	}
	line := func(e jikko.Entry) string {
		s := fmt.Sprintf("%s [%s r%d] %s", e.Path, e.Type, e.Revision, e.Title)
		if e.Retracted {
			s += " (retracted)"
		}
		if e.ReadOnly {
			s += " (read-only)"
		}
		return s
	}
	render := func(depth int) []string {
		var out []string
		dirs := map[string]int{}
		var order []string
		for _, e := range in {
			rest := strings.Split(strings.TrimPrefix(e.Path, prefix), "/")
			if depth <= 0 || len(rest) <= depth {
				out = append(out, line(e))
				continue
			}
			d := prefix + strings.Join(rest[:depth], "/") + "/"
			if dirs[d] == 0 {
				order = append(order, d)
				out = append(out, "")
			}
			dirs[d]++
		}
		j := 0
		for i := range out {
			if out[i] == "" {
				out[i] = fmt.Sprintf("%s (%d documents)", order[j], dirs[order[j]])
				j++
			}
		}
		return out
	}
	if len(in) <= limit {
		return render(0)
	}
	best := render(1)
	for d := 2; d < 16; d++ {
		r := render(d)
		if len(r) > limit {
			break
		}
		best = r
	}
	if len(best) > limit {
		best = append(best[:limit], fmt.Sprintf("... %d more lines; list a directory with jikko.tree", len(best)-limit))
	}
	return best
}

// Round executes one tool-call round: one model reply, which may contain
// several calls. It returns one JSON result per call. Results that would
// overflow the input-token budget are replaced by a budget notice.
func (t *Tools) Round(calls []ToolCall) []string {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make([]string, len(calls))
	fail := func(i int, msg string) {
		t.usage.Calls = append(t.usage.Calls, ToolCallRecord{Round: t.usage.Rounds, Tool: calls[i].Tool, Arg: callArg(calls[i]), Error: msg})
		out[i] = encodeResult(map[string]any{"error": msg})
	}
	switch {
	case t.closed:
		for i := range calls {
			fail(i, "the tool session is closed")
		}
		return out
	case t.usage.Rounds >= t.budget.Rounds:
		t.exhaust("rounds")
		for i := range calls {
			fail(i, t.submitNow(fmt.Sprintf("tool-call budget of %d rounds exhausted", t.budget.Rounds)))
		}
		return out
	case t.usage.InputTokens >= t.budget.InputTokens:
		for i := range calls {
			fail(i, t.submitNow("input token budget exhausted"))
		}
		return out
	}
	t.usage.Rounds++
	for i, c := range calls {
		res, err := t.call(c)
		if err != nil {
			fail(i, err.Error())
			continue
		}
		res["rounds_left"] = t.budget.Rounds - t.usage.Rounds
		s := encodeResult(res)
		if t.usage.InputTokens+ApproxTokens(s) > t.budget.InputTokens {
			t.exhaust("input_tokens")
			t.usage.InputTokens = t.budget.InputTokens
			fail(i, t.submitNow("this result would exceed the input token budget"))
			continue
		}
		t.usage.InputTokens += ApproxTokens(s)
		t.usage.Calls = append(t.usage.Calls, ToolCallRecord{Round: t.usage.Rounds, Tool: c.Tool, Arg: callArg(c)})
		out[i] = s
	}
	return out
}

func (t *Tools) submitNow(why string) string {
	if t.reflection {
		return why + "; give your reflection note now"
	}
	return why + "; submit your decision now"
}

func callArg(c ToolCall) string {
	switch {
	case c.Path != "":
		return c.Path
	case len(c.Paths) > 0:
		return strings.Join(c.Paths, ",")
	case c.Query != "":
		return c.Query
	case c.Ref != "":
		return c.Ref
	}
	return ""
}

func encodeResult(v map[string]any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return `{"error":"unencodable result"}`
	}
	return string(b)
}

func (t *Tools) call(c ToolCall) (map[string]any, error) {
	who := t.usage.Identity
	switch c.Tool {
	case ToolTree:
		prefix := strings.TrimPrefix(c.Path, "/")
		if prefix != "" && !strings.HasSuffix(prefix, "/") {
			prefix += "/"
		}
		return map[string]any{"revision": t.usage.TreeRevision, "entries": boundedTree(t.tree(), prefix, TreeLimit)}, nil
	case ToolRead:
		return t.readDocs([]string{c.Path})
	case ToolReadMany:
		if len(c.Paths) == 0 {
			return nil, fmt.Errorf("paths must list at least one document")
		}
		if len(c.Paths) > t.budget.ReadMany {
			return nil, fmt.Errorf("read_many takes at most %d paths, got %d", t.budget.ReadMany, len(c.Paths))
		}
		return t.readDocs(c.Paths)
	case ToolSearch:
		q := strings.ToLower(strings.TrimSpace(c.Query))
		if q == "" {
			return nil, fmt.Errorf("query must not be empty")
		}
		var matches []map[string]any
		more := false
		for _, e := range t.tree() {
			d, _ := t.read(e.Path)
			for n, l := range strings.Split(d.Content, "\n") {
				if strings.Contains(strings.ToLower(l), q) {
					if len(matches) == SearchLimit {
						more = true
						break
					}
					if len(l) > snippetBytes {
						l = l[:snippetBytes]
					}
					matches = append(matches, map[string]any{"path": e.Path, "line": n + 1, "text": l})
				}
			}
		}
		return map[string]any{"matches": matches, "truncated": more}, nil
	case ToolMentions:
		names := append([]string{who}, t.usage.Groups...)
		var paths []string
		for _, e := range t.tree() {
			d, _ := t.read(e.Path)
			for _, n := range names {
				if strings.Contains(d.Content, "@"+n) {
					paths = append(paths, e.Path)
					break
				}
			}
		}
		if len(paths) == 0 {
			return map[string]any{"documents": []any{}}, nil
		}
		return t.readDocs(paths)
	case ToolCreate, ToolUpdate, ToolRetract:
		return t.write(c)
	case ToolInspect:
		v := inspect(t.obs, c.Ref)
		if v == nil {
			return nil, fmt.Errorf("%q is not an object you can observe", c.Ref)
		}
		return map[string]any{"ref": c.Ref, "object": v}, nil
	case ToolSubmit:
		if t.reflection {
			return nil, fmt.Errorf("orders cannot be given during reflection")
		}
		return nil, fmt.Errorf("game.submit ends the turn and must be your final reply")
	}
	return nil, fmt.Errorf("unknown tool %q (tools: %s)", c.Tool, strings.Join(ToolNames, ", "))
}

// readDocs returns documents, counting each against the per-turn file budget.
func (t *Tools) readDocs(paths []string) (map[string]any, error) {
	docs := []any{}
	var missing []string
	for _, p := range paths {
		d, ok := t.read(p)
		if !ok {
			missing = append(missing, p)
			continue
		}
		if t.usage.FilesReturned >= t.budget.Files {
			t.exhaust("files")
			missing = append(missing, p+" (file budget exhausted)")
			continue
		}
		t.usage.FilesReturned++
		t.usage.FilesRead = append(t.usage.FilesRead, FileRef{Path: d.Path, Revision: d.Revision})
		docs = append(docs, d)
	}
	if len(docs) == 0 && len(missing) > 0 {
		return nil, fmt.Errorf("no readable document: %s", strings.Join(missing, ", "))
	}
	res := map[string]any{"documents": docs}
	if len(missing) > 0 {
		res["missing"] = missing
	}
	return res, nil
}

// write stages a create, update or retraction after checking it against
// what this ruler sees, including its own staged writes.
func (t *Tools) write(c ToolCall) (map[string]any, error) {
	who := t.usage.Identity
	w := jikko.Write{Path: strings.TrimPrefix(c.Path, "/"), Type: c.Type, Content: c.Content, ExpectedRevision: c.ExpectedRevision, Reason: c.Reason}
	switch c.Tool {
	case ToolCreate:
		w.Op = jikko.OpCreate
	case ToolUpdate:
		w.Op = jikko.OpUpdate
	default:
		w.Op = jikko.OpRetract
	}
	allowed := false
	for _, ws := range jikko.Workspaces(who, t.usage.Groups) {
		allowed = allowed || strings.HasPrefix(w.Path, ws)
	}
	if !allowed {
		return nil, fmt.Errorf("you may only write under %s", strings.Join(jikko.Workspaces(who, t.usage.Groups), " or "))
	}
	var cur *jikko.Doc
	if d, ok := t.read(w.Path); ok {
		cur = &d
		if d.ReadOnly {
			return nil, fmt.Errorf("document %s is read-only", w.Path)
		}
	}
	next, err := jikko.Next(cur, w, who, t.obs.Turn)
	if err != nil {
		return nil, err
	}
	t.overlay[w.Path] = &next
	t.staged = append(t.staged, w)
	return map[string]any{"ok": true, "path": w.Path, "revision": next.Revision, "note": "staged; committed after the decision barrier"}, nil
}

// SetWrites records the commit results of the staged writes.
func (t *Tools) SetWrites(r []jikko.WriteResult) {
	t.mu.Lock()
	t.usage.Writes = append([]jikko.WriteResult(nil), r...)
	t.mu.Unlock()
}

// inspect finds an observable object by id in the observation.
func inspect(o Observation, ref string) any {
	if ref == "" {
		return nil
	}
	if o.Empire != nil && o.Empire.ID == ref {
		return o.Empire
	}
	for _, p := range o.Planets {
		if p.ID == ref {
			return p
		}
	}
	for _, f := range o.Fleets {
		if f.ID == ref {
			return f
		}
	}
	for _, p := range o.OtherPlanets {
		if p.ID == ref {
			return p
		}
	}
	for _, f := range o.ForeignFleets {
		if f.ID == ref {
			return f
		}
	}
	for _, s := range o.Systems {
		if s.ID == ref {
			return s
		}
	}
	for _, r := range o.Rulers {
		if r.ID == ref {
			return r
		}
	}
	for _, a := range o.Alliances {
		if a.ID == ref {
			return a
		}
	}
	return nil
}
