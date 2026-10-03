package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
)

type Jikko struct{ Binary, Dir, Identity string }

func (j Jikko) call(ctx context.Context, args ...string) ([]byte, error) {
	bin := j.Binary
	if bin == "" {
		bin = "jikko"
	}
	args = append(args, "--dir", j.Dir, "--json")
	c := exec.CommandContext(ctx, bin, args...)
	c.Env = append(c.Environ(), "JIKKO_IDENTITY="+j.Identity)
	return c.Output()
}
func (j Jikko) Tree(ctx context.Context) (json.RawMessage, error) {
	b, e := j.call(ctx, "tree")
	return b, e
}
func (j Jikko) ReadMany(ctx context.Context, paths ...string) (json.RawMessage, error) {
	if len(paths) == 0 {
		return nil, fmt.Errorf("no paths")
	}
	a := append([]string{"show"}, paths...)
	b, e := j.call(ctx, a...)
	return b, e
}
