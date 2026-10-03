package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
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

const usage = "usage: agame [new [seed] | turn | run N | replay | observe EMPIRE TURN | suite SEED... | serve]"

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
	case "suite":
		return suite(ctx, s, args, out)
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
		e := run.RosterEntry{Empire: id, Name: w.Empires[id].Name, Agent: a.(interface{ Name() string }).Name()}
		if c, ok := a.(agent.Configured); ok {
			e.Config = c.Config()
		}
		roster = append(roster, e)
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
		reflections := make([]any, len(rec.Reflections))
		for i, x := range rec.Reflections {
			reflections[i] = x
		}
		if err := s.AppendJSONL(run.ReflectionsFile, reflections...); err != nil {
			return w, err
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

// suite runs one complete benchmark per seed (spec/benchmark.md: "A
// benchmark suite should run multiple universe seeds"), each in its own run
// directory seed-N under the suite directory, to its end condition, and
// prints every run's end and standings. Existing runs are resumed, never
// overwritten.
func suite(ctx context.Context, s run.Store, args []string, out io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: agame suite SEED...")
	}
	limit := budget().TurnLimit
	for _, a := range args {
		seed, err := strconv.ParseInt(a, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid seed %q", a)
		}
		rs := run.Store{Dir: filepath.Join(s.Dir, "seed-"+a)}
		if _, err := os.Stat(filepath.Join(rs.Dir, run.WorldFile)); err != nil {
			w, err := world.Generate(seed, names)
			if err != nil {
				return err
			}
			if err := rs.Create(w, run.Header{PromptVersion: agent.PromptVersion}); err != nil {
				return err
			}
		}
		w, err := rs.Load()
		if err != nil {
			return err
		}
		if world.CheckEnd(w, limit) == nil {
			if _, err := play(ctx, rs, limit-w.Turn, io.Discard); err != nil {
				return fmt.Errorf("seed %d: %w", seed, err)
			}
		}
		var res Result
		if err := rs.LoadJSON(run.ResultFile, &res); err != nil {
			return fmt.Errorf("seed %d has no result: %w", seed, err)
		}
		fmt.Fprintf(out, "seed %d: ended at turn %d (%s)\n", seed, res.End.Turn, res.End.Reason)
		for _, st := range res.Standings {
			fmt.Fprintf(out, "  %d. %s (%s) %s planets=%d survived=%d score=%d\n", st.Rank, st.Name, st.Empire, st.Status, st.Planets, st.Survived, st.Score)
		}
	}
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

func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
