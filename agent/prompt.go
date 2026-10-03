package agent

import (
	"encoding/json"
	"fmt"
	"strings"
)

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
	if len(o.Messages) > 0 {
		b.WriteString("messages:\n")
		for _, m := range o.Messages {
			fmt.Fprintf(&b, "from %s (turn %d): %q\n", m.From, m.Turn, m.Body)
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
