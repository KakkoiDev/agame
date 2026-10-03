package agent

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// fakeJikko writes a shell script that prints its arguments and identity as JSON.
func fakeJikko(t *testing.T, exit int) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("needs a POSIX shell")
	}
	bin := filepath.Join(t.TempDir(), "jikko")
	script := `#!/bin/sh
printf '{"identity":"%s","args":"%s"}' "$JIKKO_IDENTITY" "$*"
exit ` + string(rune('0'+exit)) + "\n"
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return bin
}

func TestJikkoTreeAndReadMany(t *testing.T) {
	j := Jikko{Binary: fakeJikko(t, 0), Dir: "/w", Identity: "cassian"}
	var out struct{ Identity, Args string }
	b, err := j.Tree(context.Background())
	if err != nil || json.Unmarshal(b, &out) != nil {
		t.Fatalf("tree: %s %v", b, err)
	}
	if out.Identity != "cassian" || out.Args != "tree --dir /w --json" {
		t.Fatalf("tree call %+v", out)
	}
	b, err = j.ReadMany(context.Background(), "plans/war.md", "memory/1.md")
	if err != nil || json.Unmarshal(b, &out) != nil {
		t.Fatalf("read_many: %s %v", b, err)
	}
	if out.Args != "show plans/war.md memory/1.md --dir /w --json" {
		t.Fatalf("read_many call %+v", out)
	}
}

func TestJikkoErrors(t *testing.T) {
	if _, err := (Jikko{Binary: fakeJikko(t, 0)}).ReadMany(context.Background()); err == nil {
		t.Fatal("ReadMany with no paths should fail")
	}
	if _, err := (Jikko{Binary: fakeJikko(t, 3)}).Tree(context.Background()); err == nil || !strings.Contains(err.Error(), "exit status 3") {
		t.Fatalf("err=%v", err)
	}
	if _, err := (Jikko{Binary: filepath.Join(t.TempDir(), "missing")}).Tree(context.Background()); err == nil {
		t.Fatal("missing binary should fail")
	}
}
