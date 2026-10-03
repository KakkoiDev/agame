package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"time"

	"github.com/KakkoiDev/agame/agent"
	"github.com/KakkoiDev/agame/engine"
	"github.com/KakkoiDev/agame/run"
	"github.com/KakkoiDev/agame/world"
)

var names = []string{"Cassian", "Malrec", "Aya", "Kael", "Iona", "Talos", "Nara", "Orion"}

const usage = "usage: agame [new [seed] | turn | run N | replay | observe EMPIRE TURN | serve]"

func main() {
	flag.Parse()
	cmd := "serve"
	if flag.NArg() > 0 {
		cmd = flag.Arg(0)
	}
	s := run.Store{Dir: env("AGAME_RUN", "run")}
	if cmd == "serve" {
		serve(s)
		return
	}
	if err := command(context.Background(), s, cmd, flag.Args()[1:], os.Stdout); err != nil {
		log.Fatal(err)
	}
}

// command runs one CLI command against the run directory.
func command(ctx context.Context, s run.Store, cmd string, args []string, out io.Writer) error {
	switch cmd {
	case "new":
		seed := int64(1)
		if len(args) > 0 {
			var err error
			seed, err = strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid seed %q: %v", args[0], err)
			}
		}
		if _, err := os.Stat(filepath.Join(s.Dir, run.WorldFile)); err == nil {
			return fmt.Errorf("%s already contains a universe; choose another AGAME_RUN directory (a new game never overwrites an existing one)", s.Dir)
		}
		w, err := world.Generate(seed, names)
		if err != nil {
			return err
		}
		if err := s.Create(w, run.Header{PromptVersion: agent.PromptVersion}); err != nil {
			return err
		}
		fmt.Fprintln(out, s.Dir)
	case "turn":
		w, err := play(ctx, s, 1, io.Discard)
		if err != nil {
			return err
		}
		fmt.Fprintln(out, w.Turn)
	case "run":
		n := 0
		if len(args) > 0 {
			var err error
			if n, err = strconv.Atoi(args[0]); err != nil || n < 1 {
				return fmt.Errorf("invalid turn count %q", args[0])
			}
		}
		if n == 0 {
			return fmt.Errorf("usage: agame run N")
		}
		_, err := play(ctx, s, n, out)
		return err
	case "replay":
		return replay(s, out)
	case "observe":
		if len(args) != 2 {
			return fmt.Errorf("usage: agame observe EMPIRE TURN")
		}
		turn, err := strconv.Atoi(args[1])
		if err != nil {
			return fmt.Errorf("invalid turn %q", args[1])
		}
		var initial world.World
		if err := s.LoadJSON(run.InitialFile, &initial); err != nil {
			return err
		}
		turns, err := s.Turns()
		if err != nil {
			return err
		}
		o, err := engine.ObservationAt(&initial, turns, turn, args[0])
		if err != nil {
			return err
		}
		fmt.Fprintf(out, "prompt %s\n%s", agent.PromptHash(o), agent.Prompt(o))
	default:
		return fmt.Errorf("%s", usage)
	}
	return nil
}

// rulers picks the decision provider for every ruler: an OpenAI-compatible
// local model when AGAME_MODEL_ENDPOINT is set, else the deterministic
// autopilot. Every ruler shares the same provider (D6).
func rulers(w *world.World) (map[string]agent.Agent, []run.RosterEntry) {
	var a agent.Agent = agent.AutopilotAgent{}
	if ep := os.Getenv("AGAME_MODEL_ENDPOINT"); ep != "" {
		a = agent.OpenAICompatible{Endpoint: ep, APIKey: os.Getenv("AGAME_API_KEY"), Model: env("AGAME_MODEL", "local")}
	}
	m := map[string]agent.Agent{}
	var roster []run.RosterEntry
	for _, id := range sortedKeys(w.Empires) {
		m[id] = a
		roster = append(roster, run.RosterEntry{Empire: id, Name: w.Empires[id].Name, Agent: a.(interface{ Name() string }).Name()})
	}
	return m, roster
}

func budget() engine.Budget {
	b := engine.CanonicalBudget()
	if v, err := strconv.Atoi(os.Getenv("AGAME_TURN_LIMIT")); err == nil && v > 0 {
		b.TurnLimit = v
	}
	if v, err := time.ParseDuration(os.Getenv("AGAME_TIMEOUT")); err == nil && v > 0 {
		b.Timeout = v
	}
	return b
}

// play advances the stored run by up to n turns through engine.Runner, the
// same code path the browser uses, committing every turn. It stops early when
// an end condition is reached and then writes the final result.
func play(ctx context.Context, s run.Store, n int, out io.Writer) (*world.World, error) {
	w, err := s.Load()
	if err != nil {
		return nil, err
	}
	agents, roster := rulers(w)
	r := &engine.Runner{World: w, Agents: agents, Budget: budget()}
	var h run.Header
	if err := s.LoadJSON(run.HeaderFile, &h); err == nil {
		h.Roster, h.Budget, h.PromptVersion = roster, r.Budget, agent.PromptVersion
		if err := s.SaveJSON(run.HeaderFile, h); err != nil {
			return w, err
		}
	}
	for i := 0; i < n; i++ {
		rec, err := r.Step(ctx)
		if err != nil {
			return w, err
		}
		decisions := make([]any, len(rec.Decisions))
		for i, d := range rec.Decisions {
			decisions[i] = d
		}
		if err := s.Commit(w, rec.Result, decisions...); err != nil {
			return w, err
		}
		fmt.Fprintf(out, "turn %d: %d accepted, %d rejected, %d events\n", rec.Turn, len(rec.Result.Accepted), len(rec.Result.Rejected), len(rec.Result.Events))
		if rec.End != nil {
			res, err := finalResult(s, w, rec.End)
			if err != nil {
				return w, err
			}
			fmt.Fprintf(out, "run ended at turn %d: %s\n", rec.End.Turn, rec.End.Reason)
			for _, st := range res.Standings {
				fmt.Fprintf(out, "%d. %s (%s) %s planets=%d score=%d\n", st.Rank, st.Name, st.Empire, st.Status, st.Planets, st.Score)
			}
			break
		}
	}
	return w, nil
}

// Result is the final record of an ended run.
type Result struct {
	End       *world.End            `json:"end"`
	StateHash string                `json:"state_hash"`
	Standings []world.Standing      `json:"standings"`
	Ops       map[string]engine.Ops `json:"agent_operation"`
}

func finalResult(s run.Store, w *world.World, end *world.End) (Result, error) {
	res := Result{End: end, StateHash: world.StateHash(w), Standings: world.Standings(w), Ops: map[string]engine.Ops{}}
	err := s.ReadJSONL(run.DecisionsFile, func(b []byte) error {
		var d engine.DecisionRecord
		if err := json.Unmarshal(b, &d); err != nil {
			return err
		}
		o := res.Ops[d.Empire]
		o.Add(d)
		res.Ops[d.Empire] = o
		return nil
	})
	if err != nil {
		return res, err
	}
	return res, s.SaveJSON(run.ResultFile, res)
}

// replay re-resolves the logged run from initial.json and checks that it
// reproduces every logged turn and the saved world.
func replay(s run.Store, out io.Writer) error {
	var initial world.World
	if err := s.LoadJSON(run.InitialFile, &initial); err != nil {
		return fmt.Errorf("replay needs %s: %w", run.InitialFile, err)
	}
	turns, err := s.Turns()
	if err != nil {
		return err
	}
	final, err := engine.Replay(&initial, turns)
	if err != nil {
		return err
	}
	// The append-only event log must be exactly the replayed events.
	var logged []string
	if err := s.ReadJSONL(run.EventsFile, func(b []byte) error { logged = append(logged, string(b)); return nil }); err != nil {
		return err
	}
	n := 0
	for _, t := range turns {
		for _, e := range t.Events {
			b, _ := json.Marshal(e)
			if n >= len(logged) || logged[n] != string(b) {
				return fmt.Errorf("%s diverges from the replay at entry %d (turn %d)", run.EventsFile, n+1, t.Turn)
			}
			n++
		}
	}
	if n != len(logged) {
		return fmt.Errorf("%s has %d entries, the replay produced %d", run.EventsFile, len(logged), n)
	}
	saved, err := s.Load()
	if err != nil {
		return err
	}
	got, want := world.StateHash(final), world.StateHash(saved)
	if got != want {
		return fmt.Errorf("replayed %d turns to state %s, but %s has %s", len(turns), got, run.WorldFile, want)
	}
	fmt.Fprintf(out, "replayed %d turns and %d events; final state %s matches %s\n", len(turns), n, got, run.WorldFile)
	return nil
}

func sortedKeys[V any](m map[string]V) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}

type row struct {
	Name    string
	Planets int
	Status  string
}
type page struct {
	Turn, Year, Month int
	Rows              []row
}

var tpl = template.Must(template.New("home").Parse(`<!doctype html><html><head><meta name=viewport content="width=device-width"><title>AGame</title><script src="https://unpkg.com/htmx.org@2.0.4"></script><style>body{font:16px system-ui;max-width:1000px;margin:auto;padding:2rem;background:#111;color:#eee}table{width:100%;border-collapse:collapse}td,th{padding:.5rem;border-bottom:1px solid #333;text-align:left}.muted{color:#999}</style></head><body><h1>AGame</h1><p>Turn {{.Turn}} · Year {{.Year}}, month {{.Month}}</p><table><tr><th>Empire</th><th>Planets</th><th>Status</th></tr>{{range .Rows}}<tr><td>{{.Name}}</td><td>{{.Planets}}</td><td>{{.Status}}</td></tr>{{end}}</table><p class=muted>Observer dashboard · objective world state</p></body></html>`))

func serve(s run.Store) {
	log.Println("AGame http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", dashboard(s)))
}

// dashboard renders into a buffer first: a failed write to one client (e.g. a disconnect) must never take the server down, and a template error yields a clean 500.
func dashboard(s run.Store) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(rw, r)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			rw.Header().Set("Allow", "GET, HEAD")
			http.Error(rw, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w, err := s.Load()
		if err != nil {
			http.Error(rw, "run not found; use agame new", 404)
			return
		}
		p := page{Turn: w.Turn, Year: w.Turn/12 + 1, Month: w.Turn%12 + 1}
		ids := make([]string, 0, len(w.Empires))
		for id := range w.Empires {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		for _, id := range ids {
			e := w.Empires[id]
			n := 0
			for _, x := range w.Planets {
				if x.OwnerID == e.ID {
					n++
				}
			}
			status := "sovereign"
			if e.Exile {
				status = "exile"
			}
			if e.Eliminated {
				status = "eliminated"
			}
			p.Rows = append(p.Rows, row{e.Name, n, status})
		}
		var buf bytes.Buffer
		if err := tpl.Execute(&buf, p); err != nil {
			log.Printf("render: %v", err)
			http.Error(rw, "render failed", 500)
			return
		}
		rw.Header().Set("Content-Type", "text/html; charset=utf-8")
		if _, err := buf.WriteTo(rw); err != nil {
			log.Printf("write: %v", err)
		}
	})
}
func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
