package media

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestCachePutIsContentAddressed(t *testing.T) {
	c := Cache{Dir: filepath.Join(t.TempDir(), "cache")}
	data := []byte("portrait-bytes")
	req := Request{Kind: "portrait", Prompt: "Cassian, stern", Model: "flux-dev", Seed: 42}
	a, err := c.Put(req, data)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	if a.Hash != hex.EncodeToString(sum[:]) || a.Path != filepath.Join(c.Dir, a.Hash) {
		t.Fatalf("asset %+v", a)
	}
	if a.Model != req.Model || a.Prompt != req.Prompt || a.Seed != req.Seed {
		t.Fatalf("provenance lost: %+v", a)
	}
	got, err := os.ReadFile(a.Path)
	if err != nil || !bytes.Equal(got, data) {
		t.Fatalf("stored %q err %v", got, err)
	}

	// Same bytes from a different request dedupe to the same file; different
	// bytes get a different address.
	b, err := c.Put(Request{Prompt: "other"}, data)
	if err != nil || b.Path != a.Path {
		t.Fatalf("dedupe failed: %+v %v", b, err)
	}
	d, err := c.Put(req, []byte("other-bytes"))
	if err != nil || d.Hash == a.Hash {
		t.Fatalf("collision: %+v %v", d, err)
	}
	entries, _ := os.ReadDir(c.Dir)
	if len(entries) != 2 {
		t.Fatalf("cache holds %d files", len(entries))
	}
}

func TestCachePutFailsWhenDirIsAFile(t *testing.T) {
	f := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(f, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := (Cache{Dir: f}).Put(Request{}, []byte("x")); err == nil {
		t.Fatal("expected error")
	}
}
