package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/KakkoiDev/agame/jikko"
)

// DecideTools lets the autopilot keep Jikko memory like any other ruler,
// through the same budgeted tools: its current plan is a task it rewrites
// every turn, and while it belongs to an alliance it shares a status task
// in the alliance workspace. Its orders are exactly Decide's.
func (a AutopilotAgent) DecideTools(ctx context.Context, o Observation, t *Tools) (Decision, error) {
	d, err := a.Decide(ctx, o)
	if err != nil || o.Empire == nil {
		return d, err
	}
	id := o.Empire.ID
	plan := fmt.Sprintf("# Current plan\nTurn %d: %s\n", o.Turn, orNone(d.Statement))
	keepTask(t, "rulers/"+id+"/plan.md", plan)
	if aid := o.Empire.AllianceID; aid != "" && o.Turn%12 == 0 {
		status := fmt.Sprintf("# %s status\nTurn %d: %d planets, %d fleets. @%s\n", id, o.Turn, len(o.Planets), len(o.Fleets), aid)
		keepTask(t, "alliances/"+aid+"/"+id+"-status.md", status)
	}
	return d, nil
}

// ReflectTools appends one line per reflection to the autopilot's durable
// memory document, never rewriting earlier lines.
func (AutopilotAgent) ReflectTools(_ context.Context, o Observation, t *Tools, triggers []string) (string, error) {
	if o.Empire == nil {
		return "", nil
	}
	note := fmt.Sprintf("Turn %d: %s.", o.Turn, strings.Join(triggers, ", "))
	p := "rulers/" + o.Empire.ID + "/memory.md"
	if d, ok := readOne(t, p); ok {
		t.Round([]ToolCall{{Tool: ToolUpdate, Path: p, ExpectedRevision: d.Revision, Content: d.Content + note + "\n"}})
	} else {
		t.Round([]ToolCall{{Tool: ToolCreate, Path: p, Type: jikko.TypeDocument, Content: "# Memory\n" + note + "\n"}})
	}
	return note, nil
}

// keepTask creates or rewrites a task document when its content changed.
func keepTask(t *Tools, path, content string) {
	d, ok := readOne(t, path)
	switch {
	case !ok:
		t.Round([]ToolCall{{Tool: ToolCreate, Path: path, Type: jikko.TypeTask, Content: content}})
	case d.Content != content && !d.Retracted:
		t.Round([]ToolCall{{Tool: ToolUpdate, Path: path, ExpectedRevision: d.Revision, Content: content}})
	}
}

// readOne reads a document through the budgeted tools.
func readOne(t *Tools, path string) (jikko.Doc, bool) {
	var r struct {
		Documents []jikko.Doc `json:"documents"`
	}
	if json.Unmarshal([]byte(t.Round([]ToolCall{{Tool: ToolRead, Path: path}})[0]), &r) != nil || len(r.Documents) == 0 {
		return jikko.Doc{}, false
	}
	return r.Documents[0], true
}
