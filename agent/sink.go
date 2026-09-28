package agent

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// JikkoSink stores generated prose through Jikko. Decision models never need
// to generate Markdown themselves.
type JikkoSink struct {
	Binary, Dir, Identity string
}

func (j JikkoSink) Write(ctx context.Context, intent WriteIntent, text string) error {
	if strings.TrimSpace(text)=="" { return nil }
	bin:=j.Binary; if bin==""{bin="jikko"}
	path:=intent.Target
	if path=="" {
		name:=strings.ReplaceAll(intent.Kind," ","-")
		path=filepath.ToSlash(filepath.Join("agent",name+".md"))
	}
	cmd:=exec.CommandContext(ctx,bin,"create",path,"--body",text)
	cmd.Dir=j.Dir
	cmd.Env=append(os.Environ(),"JIKKO_IDENTITY="+j.Identity)
	out,err:=cmd.CombinedOutput()
	if err!=nil{return fmt.Errorf("jikko write: %w: %s",err,string(out))}
	return nil
}
