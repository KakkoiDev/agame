package agent

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

// PromptVersion identifies the harness prompt format; it is recorded with
// every run (spec/benchmark.md, Reproducible universe).
const PromptVersion = "agame-prompt-2"

// PromptHash identifies exactly what a model was shown for an observation.
func PromptHash(o Observation) string {
	h := sha256.Sum256([]byte(SystemPrompt(o) + "\x00" + Prompt(o)))
	return hex.EncodeToString(h[:])
}

const decisionSchema = `Return JSON only: {"orders":[{"type":"...","actor":"...","target":"...","params":{}}],"statement":"..."}. ` +
	`Choose zero or more legal AGame orders. Use only ids that appear in the observation; never invent ids.`

// SystemPrompt is the rules contract: the decision schema plus the legal
// order types with their field shapes.
func SystemPrompt(o Observation) string {
	var b strings.Builder
	b.WriteString(decisionSchema)
	b.WriteString("\nLegal orders (type: actor -> target {params}; note):\n")
	for _, s := range o.Orders {
		fmt.Fprintf(&b, "%s: %s", s.Type, orNone(s.Actor))
		if s.Target != "" {
			fmt.Fprintf(&b, " -> %s", s.Target)
		}
		if len(s.Params) > 0 {
			keys := sortedIDs(s.Params)
			parts := make([]string, len(keys))
			for i, k := range keys {
				parts[i] = k + ": " + s.Params[k]
			}
			fmt.Fprintf(&b, " {%s}", strings.Join(parts, ", "))
		}
		if s.Note != "" {
			fmt.Fprintf(&b, "; %s", s.Note)
		}
		b.WriteByte('\n')
	}
	return b.String()
}

func orNone(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

// Prompt renders an observation compactly for small models: own assets as
// one JSON object per line and the graph as one line per system.
func Prompt(o Observation) string {
	var b strings.Builder
	if o.Empire == nil {
		fmt.Fprintf(&b, "turn %d; you have no empire\n", o.Turn)
		return b.String()
	}
	e := o.Empire
	fmt.Fprintf(&b, "turn %d; you are %s %q; exile=%t\n", o.Turn, e.ID, e.Name, e.Exile)
	t := e.Tech
	fmt.Fprintf(&b, "tech: industry=%d propulsion=%d weapons=%d shields=%d sensors=%d colonization=%d\n",
		t.Industry, t.Propulsion, t.Weapons, t.Shields, t.Sensors, t.Colonization)
	if q := e.Research; q != nil {
		fmt.Fprintf(&b, "research: %s L%d %d/%d\n", q.Kind, q.Level, q.Progress, q.Required)
	}
	b.WriteString("your planets:\n")
	for _, p := range o.Planets {
		writeJSONLine(&b, p)
	}
	b.WriteString("your fleets:\n")
	for _, f := range o.Fleets {
		writeJSONLine(&b, f)
	}
	if e.AllianceID != "" {
		fmt.Fprintf(&b, "your alliance: %s\n", e.AllianceID)
	}
	if len(o.Rulers) > 0 {
		parts := make([]string, 0, len(o.Rulers))
		for _, r := range o.Rulers {
			if r.ID != e.ID {
				parts = append(parts, fmt.Sprintf("%s=%s(%s)", r.ID, r.Name, r.Status))
			}
		}
		fmt.Fprintf(&b, "other rulers: %s\n", strings.Join(parts, " "))
	}
	if len(o.Invitations) > 0 {
		fmt.Fprintf(&b, "invited to join: %s (use alliance_join)\n", strings.Join(o.Invitations, " "))
	}
	if len(o.Alliances) > 0 {
		b.WriteString("alliances:\n")
		for _, a := range o.Alliances {
			fmt.Fprintf(&b, "%s %q members=%s", a.ID, a.Name, strings.Join(a.Members, ","))
			if len(a.Invited) > 0 {
				fmt.Fprintf(&b, " invited=%s", strings.Join(a.Invited, ","))
			}
			b.WriteByte('\n')
		}
	}
	if len(o.Hostilities) > 0 {
		ids := sortedIDs(o.Hostilities)
		parts := make([]string, len(ids))
		for i, id := range ids {
			parts[i] = fmt.Sprintf("%s(last battle turn %d)", id, o.Hostilities[id])
		}
		fmt.Fprintf(&b, "fought with: %s\n", strings.Join(parts, " "))
	}
	if len(o.Messages) > 0 {
		b.WriteString("messages delivered this turn:\n")
		for _, m := range o.Messages {
			major := ""
			if m.Major {
				major = " MAJOR"
			}
			fmt.Fprintf(&b, "from %s to %s%s: %q\n", m.From, m.To, major, m.Body)
		}
	}
	if len(o.Rejected) > 0 {
		b.WriteString("your orders rejected last turn:\n")
		for _, r := range o.Rejected {
			fmt.Fprintf(&b, "%s: %s\n", describeOrder(r.Order), r.Reason)
		}
	}
	if len(o.Events) > 0 {
		b.WriteString("last turn's events you know of:\n")
		for _, ev := range o.Events {
			fmt.Fprintf(&b, "%s", ev.Type)
			for _, x := range []string{ev.EmpireID, ev.Target, ev.Other, ev.Detail} {
				if x != "" {
					fmt.Fprintf(&b, " %s", x)
				}
			}
			b.WriteByte('\n')
			if ev.Report != nil {
				b.WriteString("report: ")
				writeJSONLine(&b, ev.Report)
			}
		}
	}
	owner := map[string]string{}
	for _, p := range o.Planets {
		owner[p.ID] = p.OwnerID + homeMark(p.Homeworld)
	}
	for _, p := range o.OtherPlanets {
		owner[p.ID] = orNone(p.OwnerID) + homeMark(p.Homeworld)
	}
	b.WriteString("graph (system: neighbors | planet=owner, -=unowned, *=homeworld):\n")
	for _, s := range o.Systems {
		ps := make([]string, len(s.Planets))
		for i, id := range s.Planets {
			ps[i] = id + "=" + owner[id]
		}
		fmt.Fprintf(&b, "%s: %s | %s", s.ID, strings.Join(s.Neighbors, " "), strings.Join(ps, " "))
		if s.Debris != nil {
			fmt.Fprintf(&b, " | debris m=%d c=%d", s.Debris.Metal, s.Debris.Crystal)
		}
		b.WriteByte('\n')
	}
	if len(o.ForeignFleets) > 0 {
		b.WriteString("foreign fleets seen:\n")
		for _, f := range o.ForeignFleets {
			fmt.Fprintf(&b, "%s owner=%s at %s size=%s\n", f.ID, f.OwnerID, f.SystemID, f.Size)
		}
	}
	return b.String()
}

func homeMark(h bool) string {
	if h {
		return "*"
	}
	return ""
}

func writeJSONLine(b *strings.Builder, v any) {
	j, err := json.Marshal(v)
	if err != nil {
		j = []byte(fmt.Sprintf("%q", err.Error()))
	}
	b.Write(j)
	b.WriteByte('\n')
}
