package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/KakkoiDev/agame/agent"
	"github.com/KakkoiDev/agame/engine"
	"github.com/KakkoiDev/agame/jikko"
	"github.com/KakkoiDev/agame/run"
	"github.com/KakkoiDev/agame/world"
)

func savedRun(t *testing.T) run.Store {
	t.Helper()
	s := run.Store{Dir: t.TempDir()}
	w, err := world.Generate(1, names)
	if err != nil {
		t.Fatal(err)
	}
	w.Turn = 25
	w.Empires["e01"].Name = "<script>alert(1)</script>"
	w.Empires["e02"].Exile = true
	w.Empires["e03"].Eliminated = true
	if err := s.Save(w); err != nil {
		t.Fatal(err)
	}
	return s
}

func get(h http.Handler, method, path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, path, nil))
	return rec
}

func TestDashboardRendersSortedEscapedRows(t *testing.T) {
	h := dashboard(savedRun(t))
	rec := get(h, "GET", "/")
	body := rec.Body.String()
	if rec.Code != 200 || !strings.HasPrefix(rec.Header().Get("Content-Type"), "text/html") {
		t.Fatalf("code %d type %q", rec.Code, rec.Header().Get("Content-Type"))
	}
	if !strings.Contains(body, "Turn 25 · Year 3, month 2") {
		t.Fatalf("date missing: %s", body)
	}
	if strings.Contains(body, "<script>alert") || !strings.Contains(body, "&lt;script&gt;") {
		t.Fatal("empire name not HTML-escaped")
	}
	if !strings.Contains(body, "exile") || !strings.Contains(body, "eliminated") {
		t.Fatal("status column missing")
	}
	// Rows must follow empire ID order, not map order.
	last := -1
	for _, n := range names {
		if n == "Malrec" {
			continue // renamed above
		}
		i := strings.Index(body, "<td>"+n+"</td>")
		if i < last {
			t.Fatalf("rows not in empire order: %s", body)
		}
		last = i
	}
	for i := 0; i < 10; i++ {
		if get(h, "GET", "/").Body.String() != body {
			t.Fatal("dashboard output is nondeterministic")
		}
	}
}

func TestDashboardStatusCodes(t *testing.T) {
	h := dashboard(savedRun(t))
	if c := get(h, "GET", "/favicon.ico").Code; c != 404 {
		t.Fatalf("unknown path -> %d", c)
	}
	if c := get(h, "POST", "/").Code; c != 405 {
		t.Fatalf("POST -> %d", c)
	}
	if c := get(dashboard(run.Store{Dir: t.TempDir()}), "GET", "/").Code; c != 404 {
		t.Fatalf("missing run -> %d", c)
	}
}

type brokenWriter struct{ h http.Header }

func (b *brokenWriter) Header() http.Header       { return b.h }
func (b *brokenWriter) WriteHeader(int)           {}
func (b *brokenWriter) Write([]byte) (int, error) { return 0, errors.New("client went away") }

func TestDashboardSurvivesClientWriteError(t *testing.T) {
	// The handler used must(tpl.Execute(...)), i.e. log.Fatal: any client
	// disconnecting mid-response terminated the whole server process.
	dashboard(savedRun(t)).ServeHTTP(&brokenWriter{h: http.Header{}}, httptest.NewRequest("GET", "/", nil))
}

func TestEnvDefault(t *testing.T) {
	t.Setenv("AGAME_TEST_X", "")
	if env("AGAME_TEST_X", "d") != "d" {
		t.Fatal("default")
	}
	t.Setenv("AGAME_TEST_X", "v")
	if env("AGAME_TEST_X", "d") != "v" {
		t.Fatal("override")
	}
}

func buildCLI(t *testing.T) string {
	t.Helper()
	if testing.Short() {
		t.Skip("builds the CLI")
	}
	bin := filepath.Join(t.TempDir(), "agame")
	if out, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	return bin
}

func TestCLINewTurnLifecycle(t *testing.T) {
	bin := buildCLI(t)
	dir := filepath.Join(t.TempDir(), "run")
	cli := func(args ...string) (string, error) {
		c := exec.Command(bin, args...)
		c.Env = append(os.Environ(), "AGAME_RUN="+dir)
		out, err := c.CombinedOutput()
		return string(out), err
	}
	if out, err := cli("new", "x42"); err == nil {
		t.Fatalf("invalid seed accepted: %s", out)
	}
	if out, err := cli("new", "42"); err != nil {
		t.Fatalf("new: %v %s", err, out)
	}
	if out, err := cli("new", "43"); err == nil || !strings.Contains(out, "already contains a universe") {
		t.Fatalf("existing universe overwritten: %v %s", err, out)
	}
	w, err := (run.Store{Dir: dir}).Load()
	if err != nil || w.Seed != 42 {
		t.Fatalf("seed %v err %v", w, err)
	}
	for want := 1; want <= 3; want++ {
		out, err := cli("turn")
		if err != nil || strings.TrimSpace(out) != strconv.Itoa(want) {
			t.Fatalf("turn %d: %q %v", want, out, err)
		}
	}
	if out, err := cli("bogus"); err == nil || !strings.Contains(out, "usage") {
		t.Fatalf("bogus command: %v %s", err, out)
	}
}

func TestRunReplayAndResult(t *testing.T) {
	s := run.Store{Dir: filepath.Join(t.TempDir(), "run")}
	ctx := context.Background()
	var out strings.Builder
	if err := command(ctx, s, "new", []string{"9"}, &out); err != nil {
		t.Fatal(err)
	}
	var h run.Header
	if err := s.LoadJSON(run.HeaderFile, &h); err != nil || h.Ruleset != world.RulesetVersion || h.Seed != 9 || h.InitialHash == "" {
		t.Fatalf("header %+v %v", h, err)
	}
	for _, bad := range [][]string{{}, {"0"}, {"x"}} {
		if err := command(ctx, s, "run", bad, &out); err == nil {
			t.Fatalf("run %v accepted", bad)
		}
	}
	t.Setenv("AGAME_TURN_LIMIT", "6")
	out.Reset()
	if err := command(ctx, s, "run", []string{"4"}, &out); err != nil {
		t.Fatal(err)
	}
	if strings.Count(out.String(), "accepted") != 4 {
		t.Fatalf("output %s", out.String())
	}
	if err := s.LoadJSON(run.HeaderFile, &h); err != nil || len(h.Roster) != 8 || h.Roster[0].Agent != "autopilot" || h.PromptVersion == "" {
		t.Fatalf("roster %+v", h)
	}
	out.Reset()
	if err := command(ctx, s, "replay", nil, &out); err != nil || !strings.Contains(out.String(), "replayed 4 turns") {
		t.Fatalf("replay: %v %s", err, out.String())
	}
	out.Reset()
	if err := command(ctx, s, "observe", []string{"e02", "3"}, &out); err != nil || !strings.Contains(out.String(), "turn 3; you are e02") {
		t.Fatalf("observe: %v %s", err, out.String())
	}
	for _, bad := range [][]string{{"e02"}, {"e02", "x"}, {"e02", "40"}, {"e99", "1"}} {
		if err := command(ctx, s, "observe", bad, &out); err == nil {
			t.Fatalf("observe %v accepted", bad)
		}
	}
	out.Reset()
	if err := command(ctx, s, "run", []string{"10"}, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "run ended at turn 6: turn_limit") || !strings.Contains(out.String(), "1. ") {
		t.Fatalf("output %s", out.String())
	}
	var res Result
	if err := s.LoadJSON(run.ResultFile, &res); err != nil || res.End.Reason != world.EndTurnLimit || len(res.Standings) != 8 || res.Ops["e00"].Turns != 6 {
		t.Fatalf("result %+v %v", res, err)
	}
	if err := command(ctx, s, "turn", nil, &out); err == nil || !strings.Contains(err.Error(), "already ended") {
		t.Fatalf("ended run advanced: %v", err)
	}
	n, want := 0, 0
	for _, o := range res.Ops {
		want += o.Turns
	}
	if err := s.ReadJSONL(run.DecisionsFile, func([]byte) error { n++; return nil }); err != nil || n != want || n < 40 {
		t.Fatalf("decision records %d %v", n, err)
	}
	n = 0
	if err := s.ReadJSONL(run.ReflectionsFile, func(b []byte) error {
		var r engine.ReflectionRecord
		n++
		if err := json.Unmarshal(b, &r); err != nil || len(r.Triggers) == 0 || !r.Offered || r.Jikko == nil || r.Jikko.Rounds > agent.ReflectionRounds {
			return fmt.Errorf("reflection %s", b)
		}
		return nil
	}); err != nil || n == 0 {
		t.Fatalf("reflections %d %v", n, err)
	}
	// A tampered event log fails the replay.
	ev := filepath.Join(s.Dir, run.EventsFile)
	orig, _ := os.ReadFile(ev)
	if err := os.WriteFile(ev, []byte(strings.Replace(string(orig), "production", "prodution", 1)), 0644); err != nil {
		t.Fatal(err)
	}
	if err := command(ctx, s, "replay", nil, &out); err == nil || !strings.Contains(err.Error(), "diverges") {
		t.Fatalf("tampered event log replayed: %v", err)
	}
	if err := os.WriteFile(ev, append(orig, orig[:strings.Index(string(orig), "\n")+1]...), 0644); err != nil {
		t.Fatal(err)
	}
	if err := command(ctx, s, "replay", nil, &out); err == nil || !strings.Contains(err.Error(), "entries") {
		t.Fatalf("extra event log entry replayed: %v", err)
	}
	if err := os.WriteFile(ev, orig, 0644); err != nil {
		t.Fatal(err)
	}
	// A world that does not match its log fails the replay.
	w, _ := s.Load()
	w.Planets[w.Empires["e00"].HomeworldID].Resources.Metal++
	if err := s.Save(w); err != nil {
		t.Fatal(err)
	}
	if err := command(ctx, s, "replay", nil, &out); err == nil || !strings.Contains(err.Error(), "but world.json has") {
		t.Fatalf("tampered world replayed: %v", err)
	}
	if err := command(ctx, run.Store{Dir: t.TempDir()}, "replay", nil, &out); err == nil {
		t.Fatal("replay without a run")
	}
	// The rulers' Jikko memory persisted across the separate run commands.
	mem, err := jikko.LoadMemory(filepath.Join(s.Dir, run.JikkoFile))
	if err != nil {
		t.Fatal(err)
	}
	plan, ok := mem.Read("e03", "rulers/e03/plan.md")
	if !ok || plan.Revision < 2 || !strings.Contains(plan.Content, "Turn 5:") {
		t.Fatalf("plan %+v", plan)
	}
	if _, ok := mem.Read("e03", "rulers/e00/plan.md"); ok {
		t.Fatal("a ruler reads another ruler's plan")
	}
	if err := s.LoadJSON(run.HeaderFile, &h); err != nil || h.JikkoRevision != 8 {
		t.Fatalf("jikko start revision %d", h.JikkoRevision)
	}
	var d engine.DecisionRecord
	lines, _ := s.Tail(run.DecisionsFile, 1)
	if err := json.Unmarshal(lines[0], &d); err != nil || d.Jikko == nil || d.Jikko.TreeRevision == 0 || len(d.Jikko.FilesRead) == 0 || d.Jikko.Identity != d.Empire {
		t.Fatalf("decision jikko record %+v", d.Jikko)
	}
}

func TestJikkoCanBeDisabled(t *testing.T) {
	t.Setenv("AGAME_JIKKO", "off")
	s := run.Store{Dir: t.TempDir()}
	ctx := context.Background()
	if err := command(ctx, s, "new", []string{"3"}, io.Discard); err != nil {
		t.Fatal(err)
	}
	if err := command(ctx, s, "run", []string{"2"}, io.Discard); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(s.Dir, run.JikkoFile)); !os.IsNotExist(err) {
		t.Fatalf("jikko.json written with Jikko off: %v", err)
	}
	var h run.Header
	if err := s.LoadJSON(run.HeaderFile, &h); err != nil || h.JikkoRevision != -1 {
		t.Fatalf("header %+v", h)
	}
}

func TestDashboardShowsDiplomacyBattlesAndRejections(t *testing.T) {
	s := run.Store{Dir: t.TempDir()}
	w, err := world.Generate(1, names)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Create(w, run.Header{}); err != nil {
		t.Fatal(err)
	}
	h := w.Planets[w.Empires["e00"].HomeworldID]
	target := w.Planets[w.Systems[h.SystemID].Planets[1]]
	target.OwnerID = "e01"
	w.Fleets["fa"] = &world.Fleet{ID: "fa", OwnerID: "e00", SystemID: h.SystemID, Ships: world.Ships{"cruiser": 4}}
	res, err := world.ResolveTurn(w, map[string][]world.Order{
		"e00": {{EmpireID: "e00", Type: "attack", Actor: "fa", Target: target.ID}},
		"e02": {{EmpireID: "e02", Type: "alliance_create", Params: map[string]any{"name": "<b>Pact</b>", "invite": []any{"e03"}}}},
		"e04": {{EmpireID: "e04", Type: "construct", Actor: "nowhere", Target: "metal_mine"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Commit(w, res); err != nil {
		t.Fatal(err)
	}
	body := get(dashboard(s), "GET", "/").Body.String()
	for _, want := range []string{"Standings", "&lt;b&gt;Pact&lt;/b&gt;", "Iona", "Cassian – Malrec", "captured", "battle", "alliance_created",
		"Rejected orders (turn 0)", `construct nowhere metal_mine`, `unknown planet &#34;nowhere&#34;`, "Score"} {
		if !strings.Contains(body, want) {
			t.Fatalf("dashboard lacks %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, "<b>Pact") {
		t.Fatal("alliance name not escaped")
	}
	// An ended run says so.
	w.Turn = world.CanonicalTurnLimit
	if err := s.Save(w); err != nil {
		t.Fatal(err)
	}
	if body := get(dashboard(s), "GET", "/").Body.String(); !strings.Contains(body, "Run ended at turn 600: turn_limit") {
		t.Fatal("end not shown")
	}
}

func TestDashboardDecisionsFilterAndObservation(t *testing.T) {
	s := run.Store{Dir: filepath.Join(t.TempDir(), "run")}
	ctx := context.Background()
	var out strings.Builder
	if err := command(ctx, s, "new", []string{"4"}, &out); err != nil {
		t.Fatal(err)
	}
	if err := command(ctx, s, "run", []string{"5"}, &out); err != nil {
		t.Fatal(err)
	}
	h := dashboard(s)
	body := get(h, "GET", "/").Body.String()
	for _, want := range []string{"Last turn decisions", "autopilot", `href="/observe?empire=e03&amp;turn=4"`, `href="/?empire=e02"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("dashboard lacks %q:\n%s", want, body)
		}
	}
	f := get(h, "GET", "/?empire=e02").Body.String()
	if !strings.Contains(f, "showing Aya") || strings.Contains(f, `observe?empire=e03&amp;turn`) || !strings.Contains(f, `observe?empire=e02&amp;turn=4`) {
		t.Fatalf("filtered dashboard:\n%s", f)
	}
	rec := get(h, "GET", "/observe?empire=e03&turn=2")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "Kael at turn 2") || !strings.Contains(rec.Body.String(), "turn 2; you are e03") {
		t.Fatalf("observe %d:\n%s", rec.Code, rec.Body.String())
	}
	for path, code := range map[string]int{"/observe?empire=e99&turn=1": 404, "/observe?empire=e03&turn=x": 400, "/observe?empire=e03&turn=99": 404} {
		if c := get(h, "GET", path).Code; c != code {
			t.Fatalf("%s -> %d, want %d", path, c, code)
		}
	}
}

func TestSuiteRunsEverySeedToItsEnd(t *testing.T) {
	s := run.Store{Dir: t.TempDir()}
	t.Setenv("AGAME_TURN_LIMIT", "5")
	var out strings.Builder
	if err := command(context.Background(), s, "suite", []string{"3", "4"}, &out); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"seed 3: ended at turn 5 (turn_limit)", "seed 4: ended at turn 5 (turn_limit)", "  1. "} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("suite output lacks %q:\n%s", want, out.String())
		}
	}
	// Running it again resumes nothing and reports the same results.
	var again strings.Builder
	if err := command(context.Background(), s, "suite", []string{"3", "4"}, &again); err != nil || again.String() != out.String() {
		t.Fatalf("rerun: %v\n%s", err, again.String())
	}
	if err := command(context.Background(), run.Store{Dir: filepath.Join(s.Dir, "seed-3")}, "replay", nil, &out); err != nil {
		t.Fatal(err)
	}
	for _, bad := range [][]string{{}, {"x"}} {
		if err := command(context.Background(), s, "suite", bad, &out); err == nil {
			t.Fatalf("suite %v accepted", bad)
		}
	}
}
