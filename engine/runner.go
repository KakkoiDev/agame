// Package engine runs the canonical turn loop (spec/agents.md, Turn
// lifecycle): it freezes S(t), invokes every ruler against its own
// observation, closes the decision barrier, resolves the turn, checks the end
// conditions and produces the audit records of spec/benchmark.md.
package engine

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/KakkoiDev/agame/agent"
	"github.com/KakkoiDev/agame/world"
)

// DefaultRepairs is the canonical repair budget for malformed or invalid
// structured output (spec/agents.md, Small-model budgets; D27).
const DefaultRepairs = 2

// Budget holds the per-ruler, per-turn limits of a run. They are benchmark
// settings, not engine laws, and are recorded with the run.
type Budget struct {
	// Timeout bounds one ruler's whole decision, repairs included; 0 means
	// no limit.
	Timeout time.Duration `json:"timeout_ns"`
	// Repairs is the maximum number of repair attempts; nil-equivalent
	// negative values mean none. Use DefaultRepairs for the canonical 2.
	Repairs int `json:"repairs"`
	// TurnLimit ends the run (D30); 0 means the canonical 600.
	TurnLimit int `json:"turn_limit"`
}

// CanonicalBudget is the canonical small-model benchmark budget.
func CanonicalBudget() Budget {
	return Budget{Timeout: 5 * time.Minute, Repairs: DefaultRepairs, TurnLimit: world.CanonicalTurnLimit}
}

// Attempt is one model invocation within a decision.
type Attempt struct {
	Raw   string `json:"raw,omitempty"`
	Error string `json:"error,omitempty"`
	// Problem is the validation problem fed back to the next repair.
	Problem string `json:"problem,omitempty"`
}

// DecisionRecord is the audit record of one ruler's turn (spec/benchmark.md,
// Decision record). It never contains hidden reasoning.
type DecisionRecord struct {
	Turn       int               `json:"turn"`
	Invocation int               `json:"invocation"`
	Empire     string            `json:"empire"`
	Agent      string            `json:"agent"`
	PromptHash string            `json:"prompt_hash"`
	Messages   int               `json:"messages_available"`
	Attempts   []Attempt         `json:"attempts"`
	Repairs    int               `json:"repairs"`
	Orders     []world.Order     `json:"orders"`
	Statement  string            `json:"statement,omitempty"`
	Accepted   int               `json:"accepted"`
	Rejected   []world.Rejection `json:"rejected,omitempty"`
	// Failure is "timeout", "malformed" or "error" when the ruler ended
	// with zero orders because of an agent failure.
	Failure   string `json:"failure,omitempty"`
	LatencyMS int64  `json:"latency_ms"`
}

// TurnRecord is everything one turn produced.
type TurnRecord struct {
	Turn        int                `json:"turn"`
	Decisions   []DecisionRecord   `json:"decisions"`
	Result      world.TurnResult   `json:"result"`
	Reflections []ReflectionRecord `json:"reflections,omitempty"`
	End         *world.End         `json:"end,omitempty"`
}

// Ops are the per-ruler agent-operation metrics (spec/benchmark.md).
type Ops struct {
	Turns      int   `json:"turns"`
	Invalid    int   `json:"invalid_outputs"`
	Repairs    int   `json:"repairs"`
	Timeouts   int   `json:"timeouts"`
	Errors     int   `json:"errors"`
	ZeroOrders int   `json:"zero_order_turns"`
	Orders     int   `json:"orders"`
	Rejected   int   `json:"rejected_orders"`
	LatencyMS  int64 `json:"latency_ms"`
}

// Add accumulates one decision record.
func (o *Ops) Add(d DecisionRecord) {
	o.Turns++
	o.Repairs += d.Repairs
	for _, a := range d.Attempts {
		if a.Problem != "" {
			o.Invalid++
		}
	}
	switch d.Failure {
	case "timeout":
		o.Timeouts++
	case "error":
		o.Errors++
	}
	if len(d.Orders) == 0 {
		o.ZeroOrders++
	}
	o.Orders += len(d.Orders)
	o.Rejected += len(d.Rejected)
	o.LatencyMS += d.LatencyMS
}

type Runner struct {
	World  *world.World
	Agents map[string]agent.Agent
	Budget Budget
	// Now is the clock used for latency; nil means time.Now. Latency is the
	// only nondeterministic field of a record and never affects the world.
	Now func() time.Time
}

// Turn plays one turn and returns the engine result.
func (r *Runner) Turn(ctx context.Context) (world.TurnResult, error) {
	rec, err := r.Step(ctx)
	return rec.Result, err
}

// Step plays one turn: every living ruler with an agent decides, in empire-ID
// order, from its own observation of the same frozen S(t); then the turn is
// resolved and the end conditions are checked on S(t+1). A run that has
// already ended is not advanced.
func (r *Runner) Step(ctx context.Context) (TurnRecord, error) {
	w := r.World
	if end := world.CheckEnd(w, r.Budget.TurnLimit); end != nil {
		return TurnRecord{Turn: w.Turn, End: end}, fmt.Errorf("run already ended at turn %d: %s", end.Turn, end.Reason)
	}
	rec := TurnRecord{Turn: w.Turn}
	ids := make([]string, 0, len(w.Empires))
	for id, e := range w.Empires {
		if !e.Eliminated {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	submitted := map[string][]world.Order{}
	for _, id := range ids {
		a := r.Agents[id]
		if a == nil {
			continue
		}
		d := r.decide(ctx, a, id, len(rec.Decisions))
		submitted[id] = d.Orders
		rec.Decisions = append(rec.Decisions, d)
	}
	before := fleetValues(w)
	res, err := world.ResolveTurn(w, submitted)
	if err != nil {
		return rec, fmt.Errorf("resolve: %w", err)
	}
	rec.Result = res
	for i := range rec.Decisions {
		d := &rec.Decisions[i]
		for _, x := range res.Rejected {
			if x.EmpireID == d.Empire {
				d.Rejected = append(d.Rejected, x)
			}
		}
		d.Accepted = len(d.Orders) - len(d.Rejected)
	}
	rec.End = world.CheckEnd(w, r.Budget.TurnLimit)
	if rec.End == nil {
		rec.Reflections = r.reflect(ctx, ReflectionTriggers(before, w, res.Events))
	}
	return rec, nil
}

func (r *Runner) now() time.Time {
	if r.Now != nil {
		return r.Now()
	}
	return time.Now()
}

// decide invokes one ruler with the repair loop of spec/agents.md: malformed
// output, or orders that fail validation against S(t), are fed back to a
// Repairer up to Budget.Repairs times. After that a ruler whose output is
// still malformed, or that failed or timed out, submits zero orders; a
// well-formed decision is submitted as is and the engine rejects its invalid
// orders with reasons.
func (r *Runner) decide(ctx context.Context, a agent.Agent, id string, invocation int) DecisionRecord {
	w := r.World
	obs := agent.Observe(w, id)
	rec := DecisionRecord{Turn: w.Turn, Invocation: invocation, Empire: id, Agent: agentName(a), PromptHash: agent.PromptHash(obs), Messages: len(obs.Messages)}
	if r.Budget.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, r.Budget.Timeout)
		defer cancel()
	}
	start := r.now()

	d, err := call(ctx, func(ctx context.Context) (agent.Decision, error) { return a.Decide(ctx, obs) })
	for attempt := 0; ; attempt++ {
		at := Attempt{Raw: d.Raw}
		var malformed *agent.MalformedError
		problem := ""
		switch {
		case err == nil:
			fixEmpire(d.Orders, id)
			problem = invalidOrders(w, d.Orders)
		case errors.As(err, &malformed):
			at.Raw = malformed.Raw
			problem = malformed.Err.Error()
		}
		if err != nil {
			at.Error = err.Error()
		}
		at.Problem = problem
		rec.Attempts = append(rec.Attempts, at)
		rep, canRepair := a.(agent.Repairer)
		if problem == "" || !canRepair || attempt >= r.Budget.Repairs || ctx.Err() != nil {
			break
		}
		rec.Repairs++
		prev := d
		prev.Raw = at.Raw
		d, err = call(ctx, func(ctx context.Context) (agent.Decision, error) { return rep.Repair(ctx, obs, prev, problem) })
	}
	switch {
	case err == nil:
		rec.Orders, rec.Statement = d.Orders, d.Statement
	case errors.Is(err, context.DeadlineExceeded) || ctx.Err() != nil:
		rec.Failure = "timeout"
	case errors.As(err, new(*agent.MalformedError)):
		rec.Failure = "malformed"
	default:
		rec.Failure = "error"
	}
	if rec.Orders == nil {
		rec.Orders = []world.Order{}
	}
	rec.LatencyMS = r.now().Sub(start).Milliseconds()
	return rec
}

// call runs one agent invocation but stops waiting when ctx is done, so a
// ruler that ignores its context still cannot stall the run.
func call(ctx context.Context, f func(context.Context) (agent.Decision, error)) (agent.Decision, error) {
	type out struct {
		d   agent.Decision
		err error
	}
	if ctx.Done() == nil { // no deadline or cancellation: call inline
		return f(ctx)
	}
	ch := make(chan out, 1)
	go func() {
		d, err := f(ctx)
		ch <- out{d, err}
	}()
	select {
	case o := <-ch:
		return o.d, o.err
	case <-ctx.Done():
		return agent.Decision{}, ctx.Err()
	}
}

// fixEmpire stamps every order with the deciding empire: a ruler can only
// ever order for itself.
func fixEmpire(orders []world.Order, id string) {
	for i := range orders {
		orders[i].EmpireID = id
	}
}

// invalidOrders lists the orders that fail validation against S(t), in the
// words the engine would use.
func invalidOrders(w *world.World, orders []world.Order) string {
	var probs []string
	for i, o := range orders {
		if o.Type == "" {
			probs = append(probs, fmt.Sprintf("order %d: missing type", i+1))
			continue
		}
		if err := world.ValidateOrder(w, o); err != nil {
			probs = append(probs, fmt.Sprintf("order %d (%s %s %s): %v", i+1, o.Type, o.Actor, o.Target, err))
		}
	}
	return strings.Join(probs, "; ")
}

func agentName(a agent.Agent) string {
	if n, ok := a.(interface{ Name() string }); ok {
		return n.Name()
	}
	return fmt.Sprintf("%T", a)
}
