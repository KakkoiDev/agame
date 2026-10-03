package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/KakkoiDev/agame/agent"
	"github.com/KakkoiDev/agame/engine"
	"github.com/KakkoiDev/agame/run"
	"github.com/KakkoiDev/agame/world"
)

// recentTurns is how many logged turns the dashboard reads for its history.
const recentTurns = 12

type row struct {
	Rank       int
	Name, ID   string
	Planets    int
	Status     string
	Alliance   string
	Production string
	FleetValue int
	Tech       int
	Score      int
}
type allianceRow struct {
	ID, Name, Members string
}
type warRow struct {
	Pair string
	Last int
}
type eventRow struct {
	Turn               int
	Type, Who, Details string
}
type rejectRow struct {
	Empire, Order, Reason string
}
type decisionRow struct {
	ID, Empire, Agent, Orders, Statement, Failure string
	Accepted, Rejected, Repairs                   int
	Turn                                          int
}
type page struct {
	Filter, FilterName string
	Decisions          []decisionRow
	Turn, Year, Month  int
	Rows               []row
	Alliances          []allianceRow
	Wars               []warRow
	Battles            []eventRow
	Diplomacy          []eventRow
	Rejected           []rejectRow
	RejectedTurn       int
	End                *world.End
}

var tpl = template.Must(template.New("home").Parse(`<!doctype html><html><head><meta name=viewport content="width=device-width"><title>AGame</title><script src="https://unpkg.com/htmx.org@2.0.4"></script><style>body{font:16px system-ui;max-width:1000px;margin:auto;padding:2rem;background:#111;color:#eee}table{width:100%;border-collapse:collapse;margin-bottom:1.5rem}td,th{padding:.5rem;border-bottom:1px solid #333;text-align:left}.muted{color:#999}a{color:#9cf}pre{white-space:pre-wrap;font-size:.85rem}.end{border:1px solid #9fd89f;padding:.75rem;border-radius:8px}h2{font-size:1.1rem;margin-top:2rem}@media(max-width:600px){body{padding:1rem}table{font-size:.85rem}}</style></head><body><h1>AGame</h1><p>Turn {{.Turn}} · Year {{.Year}}, month {{.Month}}{{if .Filter}} · showing {{.FilterName}} (<a href="/">all empires</a>){{end}}</p>
{{with .End}}<p class=end>Run ended at turn {{.Turn}}: {{.Reason}}</p>{{end}}
<h2>Standings</h2><table><tr><th>#</th><th>Empire</th><th>Planets</th><th>Status</th><th>Alliance</th><th>Production M/C/D</th><th>Fleet value</th><th>Tech</th><th>Score</th></tr>{{range .Rows}}<tr><td>{{.Rank}}</td><td><a href="/?empire={{.ID}}">{{.Name}}</a></td><td>{{.Planets}}</td><td>{{.Status}}</td><td>{{.Alliance}}</td><td>{{.Production}}</td><td>{{.FleetValue}}</td><td>{{.Tech}}</td><td>{{.Score}}</td></tr>{{end}}</table>
<h2>Alliances</h2>{{if .Alliances}}<table><tr><th>Alliance</th><th>Name</th><th>Members</th></tr>{{range .Alliances}}<tr><td>{{.ID}}</td><td>{{.Name}}</td><td>{{.Members}}</td></tr>{{end}}</table>{{else}}<p class=muted>No alliances.</p>{{end}}
<h2>Hostilities</h2>{{if .Wars}}<table><tr><th>Empires</th><th>Last battle</th></tr>{{range .Wars}}<tr><td>{{.Pair}}</td><td>turn {{.Last}}</td></tr>{{end}}</table>{{else}}<p class=muted>No battles yet.</p>{{end}}
<h2>Recent battles</h2>{{if .Battles}}<table><tr><th>Turn</th><th>Event</th><th>Who</th><th>Details</th></tr>{{range .Battles}}<tr><td>{{.Turn}}</td><td>{{.Type}}</td><td>{{.Who}}</td><td>{{.Details}}</td></tr>{{end}}</table>{{else}}<p class=muted>No recent battles.</p>{{end}}
<h2>Recent diplomacy</h2>{{if .Diplomacy}}<table><tr><th>Turn</th><th>Event</th><th>Who</th><th>Details</th></tr>{{range .Diplomacy}}<tr><td>{{.Turn}}</td><td>{{.Type}}</td><td>{{.Who}}</td><td>{{.Details}}</td></tr>{{end}}</table>{{else}}<p class=muted>No recent diplomacy.</p>{{end}}
<h2>Last turn decisions</h2>{{if .Decisions}}<table><tr><th>Empire</th><th>Agent</th><th>Orders</th><th>Accepted</th><th>Rejected</th><th>Repairs</th><th>Failure</th><th>Statement</th><th>Knew</th></tr>{{range .Decisions}}<tr><td>{{.Empire}}</td><td>{{.Agent}}</td><td>{{.Orders}}</td><td>{{.Accepted}}</td><td>{{.Rejected}}</td><td>{{.Repairs}}</td><td>{{.Failure}}</td><td>{{.Statement}}</td><td><a href="/observe?empire={{.ID}}&amp;turn={{.Turn}}">observation</a></td></tr>{{end}}</table>{{else}}<p class=muted>No decision records yet.</p>{{end}}
<h2>Rejected orders{{if .Rejected}} (turn {{.RejectedTurn}}){{end}}</h2>{{if .Rejected}}<table><tr><th>Empire</th><th>Order</th><th>Reason</th></tr>{{range .Rejected}}<tr><td>{{.Empire}}</td><td>{{.Order}}</td><td>{{.Reason}}</td></tr>{{end}}</table>{{else}}<p class=muted>No rejected orders last turn.</p>{{end}}
<p class=muted>Observer dashboard · objective world state</p></body></html>`))

func serve(s run.Store) {
	log.Println("AGame http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", dashboard(s)))
}

var battleEvents = map[string]bool{"battle": true, "captured": true, "attack_repulsed": true, "war_began": true, "eliminated": true}
var diplomacyEvents = map[string]bool{"alliance_created": true, "alliance_joined": true, "alliance_left": true, "alliance_dissolved": true,
	"alliance_invited": true, "treaty_breach": true, "transfer": true, "message_sent": true}

// dashboard renders into a buffer first: a failed write to one client (e.g. a disconnect) must never take the server down, and a template error yields a clean 500.
func dashboard(s run.Store) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" && r.URL.Path != "/observe" {
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
		var buf bytes.Buffer
		if r.URL.Path == "/observe" {
			o, status, err := historicalObservation(s, w, r.URL.Query().Get("empire"), r.URL.Query().Get("turn"))
			if err != nil {
				http.Error(rw, err.Error(), status)
				return
			}
			err = observeTpl.Execute(&buf, o)
			if err != nil {
				log.Printf("render: %v", err)
				http.Error(rw, "render failed", 500)
				return
			}
			rw.Header().Set("Content-Type", "text/html; charset=utf-8")
			if _, err := buf.WriteTo(rw); err != nil {
				log.Printf("write: %v", err)
			}
			return
		}
		p, err := buildPage(s, w, r.URL.Query().Get("empire"))
		if err != nil {
			log.Printf("dashboard: %v", err)
		}
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

// buildPage gathers the observer view. Log problems are reported but never
// hide the world state itself.
func buildPage(s run.Store, w *world.World, filter string) (page, error) {
	p := page{Turn: w.Turn, Year: w.Turn/12 + 1, Month: w.Turn%12 + 1, End: world.CheckEnd(w, 0)}
	name := func(id string) string {
		if e := w.Empires[id]; e != nil {
			return e.Name
		}
		return id
	}
	if w.Empires[filter] != nil {
		p.Filter, p.FilterName = filter, name(filter)
	}
	involves := func(ids ...string) bool {
		if p.Filter == "" {
			return true
		}
		for _, id := range ids {
			if id == p.Filter {
				return true
			}
		}
		return false
	}
	byID := map[string]world.Standing{}
	for _, st := range world.Standings(w) {
		byID[st.Empire] = st
	}
	for _, id := range sortedKeys(w.Empires) { // empire-ID order keeps rows stable while ranks move
		st := byID[id]
		pr := st.Production
		p.Rows = append(p.Rows, row{Rank: st.Rank, Name: st.Name, ID: id, Planets: st.Planets, Status: st.Status, Alliance: st.Alliance,
			Production: fmt.Sprintf("%d/%d/%d", pr.Metal, pr.Crystal, pr.Deuterium), FleetValue: st.FleetValue, Tech: st.TechLevels, Score: st.Score})
	}
	for _, id := range world.AllianceIDs(w) {
		a := w.Alliances[id]
		ms := make([]string, len(a.Members))
		for i, m := range a.Members {
			ms[i] = name(m)
		}
		p.Alliances = append(p.Alliances, allianceRow{ID: a.ID, Name: a.Name, Members: strings.Join(ms, ", ")})
	}
	for _, k := range sortedKeys(w.Hostilities) {
		a, b, _ := strings.Cut(k, "|")
		p.Wars = append(p.Wars, warRow{Pair: name(a) + " – " + name(b), Last: w.Hostilities[k]})
	}
	for _, id := range sortedKeys(w.Empires) {
		if !involves(id) {
			continue
		}
		for _, x := range w.Empires[id].Rejected {
			p.Rejected = append(p.Rejected, rejectRow{Empire: name(id), Order: strings.TrimSpace(x.Type + " " + x.Actor + " " + x.Target), Reason: x.Reason})
		}
	}
	p.RejectedTurn = w.Turn - 1
	lines, err := s.Tail(run.TurnsFile, recentTurns)
	if err != nil {
		return p, err
	}
	for i := len(lines) - 1; i >= 0; i-- { // newest first
		var t world.TurnResult
		if err := json.Unmarshal(lines[i], &t); err != nil {
			return p, err
		}
		for _, e := range t.Events {
			who := name(e.EmpireID)
			if e.Other != "" {
				who += " → " + name(e.Other)
			}
			if !involves(e.EmpireID, e.Other) {
				continue
			}
			ev := eventRow{Turn: e.Turn, Type: e.Type, Who: who, Details: strings.TrimSpace(e.Target + " " + e.Detail)}
			switch {
			case battleEvents[e.Type]:
				p.Battles = append(p.Battles, ev)
			case diplomacyEvents[e.Type]:
				p.Diplomacy = append(p.Diplomacy, ev)
			}
		}
	}
	dl, err := s.Tail(run.DecisionsFile, len(w.Empires))
	if err != nil {
		return p, err
	}
	for _, b := range dl {
		var d engine.DecisionRecord
		if err := json.Unmarshal(b, &d); err != nil {
			return p, err
		}
		if d.Turn != w.Turn-1 || !involves(d.Empire) {
			continue
		}
		orders := make([]string, len(d.Orders))
		for i, o := range d.Orders {
			orders[i] = strings.TrimSpace(o.Type + " " + o.Actor + " " + o.Target)
		}
		p.Decisions = append(p.Decisions, decisionRow{ID: d.Empire, Empire: name(d.Empire), Agent: d.Agent, Orders: strings.Join(orders, "; "), Statement: d.Statement,
			Failure: d.Failure, Accepted: d.Accepted, Rejected: len(d.Rejected), Repairs: d.Repairs, Turn: d.Turn})
	}
	return p, nil
}

var observeTpl = template.Must(template.New("observe").Parse(`<!doctype html><html><head><meta name=viewport content="width=device-width"><title>AGame observation</title><style>body{font:16px system-ui;max-width:1000px;margin:auto;padding:2rem;background:#111;color:#eee}a{color:#9cf}pre{white-space:pre-wrap;font-size:.85rem;border:1px solid #333;padding:1rem;border-radius:8px}.muted{color:#999}</style></head><body><p><a href="/">← dashboard</a></p><h1>{{.Name}} at turn {{.Turn}}</h1><p class=muted>What this ruler's engine observation contained then, rebuilt by replaying the run. Prompt hash {{.Hash}}. This is the ruler's knowledge at that time, not today's truth.</p><pre>{{.Prompt}}</pre></body></html>`))

type observation struct {
	Name, Hash, Prompt string
	Turn               int
}

// historicalObservation replays the run to the start of turn and renders
// what the empire observed (spec/dashboard.md, Historical knowledge view).
func historicalObservation(s run.Store, w *world.World, eid, turnParam string) (observation, int, error) {
	e := w.Empires[eid]
	if e == nil {
		return observation{}, http.StatusNotFound, fmt.Errorf("unknown empire %q", eid)
	}
	turn, err := strconv.Atoi(turnParam)
	if err != nil {
		return observation{}, http.StatusBadRequest, fmt.Errorf("invalid turn %q", turnParam)
	}
	var initial world.World
	if err := s.LoadJSON(run.InitialFile, &initial); err != nil {
		return observation{}, http.StatusNotFound, fmt.Errorf("this run has no initial state to replay")
	}
	turns, err := s.Turns()
	if err != nil {
		return observation{}, http.StatusInternalServerError, err
	}
	o, err := engine.ObservationAt(&initial, turns, turn, eid)
	if err != nil {
		return observation{}, http.StatusNotFound, err
	}
	return observation{Name: e.Name, Turn: turn, Hash: agent.PromptHash(o), Prompt: agent.Prompt(o)}, 0, nil
}
