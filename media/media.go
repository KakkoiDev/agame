package media

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
)

type Request struct {
	Kind, Prompt, Model string
	Seed                int64
}
type Asset struct {
	Hash, Path, Model, Prompt string
	Seed                      int64
}
type Generator interface {
	Generate(context.Context, Request) ([]byte, error)
}
type Cache struct{ Dir string }

func (c Cache) Put(r Request, b []byte) (Asset, error) {
	h := sha256.Sum256(b)
	id := hex.EncodeToString(h[:])
	if err := os.MkdirAll(c.Dir, 0755); err != nil {
		return Asset{}, err
	}
	p := filepath.Join(c.Dir, id)
	if err := os.WriteFile(p, b, 0644); err != nil {
		return Asset{}, err
	}
	return Asset{id, p, r.Model, r.Prompt, r.Seed}, nil
}
