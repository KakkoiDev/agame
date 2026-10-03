package world

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

// RulesetVersion names the rules this engine implements. Any change that can
// alter the trajectory of a seeded run must change it (spec/decisions.md,
// Status; spec/benchmark.md, Reproducible universe).
const RulesetVersion = "agame-v1.1"

// StateHash is the SHA-256 of the world's canonical JSON encoding (maps are
// encoded with sorted keys), so equal states hash equally across processes
// and JSON round trips.
func StateHash(w *World) string {
	b, err := json.Marshal(w)
	if err != nil {
		return "unhashable: " + err.Error()
	}
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}
