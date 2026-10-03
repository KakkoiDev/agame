package main

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

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
