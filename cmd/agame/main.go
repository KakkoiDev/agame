package main

import (
	"bytes"
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"

	"github.com/KakkoiDev/agame/run"
	"github.com/KakkoiDev/agame/world"
)

var names=[]string{"Cassian","Malrec","Aya","Kael","Iona","Talos","Nara","Orion"}

func main(){flag.Parse();cmd:="serve";if flag.NArg()>0{cmd=flag.Arg(0)};dir:=env("AGAME_RUN","run");s:=run.Store{Dir:dir};switch cmd{
case"new":seed:=int64(1);if flag.NArg()>1{var err error;seed,err=strconv.ParseInt(flag.Arg(1),10,64);if err!=nil{log.Fatalf("invalid seed %q: %v",flag.Arg(1),err)}};if _,err:=os.Stat(filepath.Join(dir,"world.json"));err==nil{log.Fatalf("%s already contains a universe; choose another AGAME_RUN directory (a new game never overwrites an existing one)",dir)};w,err:=world.Generate(seed,names);must(err);must(s.Save(w));fmt.Println(dir)
case"turn":w,err:=s.Load();must(err);res,err:=world.ResolveTurn(w,map[string][]world.Order{});must(err);must(s.Append(res.Events));must(s.Save(w));fmt.Println(w.Turn)
case"serve":serve(s)
default:log.Fatal("usage: agame [new [seed]|turn|serve]")}}
type row struct{Name string;Planets int;Status string}
type page struct{Turn,Year,Month int;Rows []row}
var tpl=template.Must(template.New("home").Parse(`<!doctype html><html><head><meta name=viewport content="width=device-width"><title>AGame</title><script src="https://unpkg.com/htmx.org@2.0.4"></script><style>body{font:16px system-ui;max-width:1000px;margin:auto;padding:2rem;background:#111;color:#eee}table{width:100%;border-collapse:collapse}td,th{padding:.5rem;border-bottom:1px solid #333;text-align:left}.muted{color:#999}</style></head><body><h1>AGame</h1><p>Turn {{.Turn}} · Year {{.Year}}, month {{.Month}}</p><table><tr><th>Empire</th><th>Planets</th><th>Status</th></tr>{{range .Rows}}<tr><td>{{.Name}}</td><td>{{.Planets}}</td><td>{{.Status}}</td></tr>{{end}}</table><p class=muted>Observer dashboard · objective world state</p></body></html>`))
func serve(s run.Store){log.Println("AGame http://localhost:8080");log.Fatal(http.ListenAndServe(":8080",dashboard(s)))}
// dashboard renders into a buffer first: a failed write to one client (e.g. a disconnect) must never take the server down, and a template error yields a clean 500.
func dashboard(s run.Store)http.Handler{return http.HandlerFunc(func(rw http.ResponseWriter,r *http.Request){if r.URL.Path!="/"{http.NotFound(rw,r);return};if r.Method!=http.MethodGet&&r.Method!=http.MethodHead{rw.Header().Set("Allow","GET, HEAD");http.Error(rw,"method not allowed",http.StatusMethodNotAllowed);return};w,err:=s.Load();if err!=nil{http.Error(rw,"run not found; use agame new",404);return};p:=page{Turn:w.Turn,Year:w.Turn/12+1,Month:w.Turn%12+1};ids:=make([]string,0,len(w.Empires));for id:=range w.Empires{ids=append(ids,id)};sort.Strings(ids);for _,id:=range ids{e:=w.Empires[id];n:=0;for _,x:=range w.Planets{if x.OwnerID==e.ID{n++}};status:="sovereign";if e.Exile{status="exile"};if e.Eliminated{status="eliminated"};p.Rows=append(p.Rows,row{e.Name,n,status})};var buf bytes.Buffer;if err:=tpl.Execute(&buf,p);err!=nil{log.Printf("render: %v",err);http.Error(rw,"render failed",500);return};rw.Header().Set("Content-Type","text/html; charset=utf-8");if _,err:=buf.WriteTo(rw);err!=nil{log.Printf("write: %v",err)}})}
func env(k,d string)string{if v:=os.Getenv(k);v!=""{return v};return d}
func must(err error){if err!=nil{log.Fatal(err)}}
