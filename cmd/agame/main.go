package main

import (
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/KakkoiDev/agame/run"
	"github.com/KakkoiDev/agame/world"
)

var names=[]string{"Cassian","Malrec","Aya","Kael","Iona","Talos","Nara","Orion"}

func main(){flag.Parse();cmd:="serve";if flag.NArg()>0{cmd=flag.Arg(0)};dir:=env("AGAME_RUN","run");s:=run.Store{Dir:dir};switch cmd{
case"new":seed:=int64(1);if flag.NArg()>1{seed,_=strconv.ParseInt(flag.Arg(1),10,64)};w,err:=world.Generate(seed,names);must(err);must(s.Save(w));fmt.Println(dir)
case"turn":w,err:=s.Load();must(err);res,err:=world.ResolveTurn(w,map[string][]world.Order{});must(err);must(s.Append(res.Events));must(s.Save(w));fmt.Println(w.Turn)
case"serve":serve(s)
default:log.Fatal("usage: agame [new [seed]|turn|serve]")}}
type row struct{Name string;Planets int;Status string}
type page struct{Turn,Year,Month int;Rows []row}
var tpl=template.Must(template.New("home").Parse(`<!doctype html><html><head><meta name=viewport content="width=device-width"><title>AGame</title><script src="https://unpkg.com/htmx.org@2.0.4"></script><style>body{font:16px system-ui;max-width:1000px;margin:auto;padding:2rem;background:#111;color:#eee}table{width:100%;border-collapse:collapse}td,th{padding:.5rem;border-bottom:1px solid #333;text-align:left}.muted{color:#999}</style></head><body><h1>AGame</h1><p>Turn {{.Turn}} · Year {{.Year}}, month {{.Month}}</p><table><tr><th>Empire</th><th>Planets</th><th>Status</th></tr>{{range .Rows}}<tr><td>{{.Name}}</td><td>{{.Planets}}</td><td>{{.Status}}</td></tr>{{end}}</table><p class=muted>Observer dashboard · objective world state</p></body></html>`))
func serve(s run.Store){http.HandleFunc("/",func(rw http.ResponseWriter,r *http.Request){w,err:=s.Load();if err!=nil{http.Error(rw,"run not found; use agame new",404);return};p:=page{Turn:w.Turn,Year:w.Turn/12+1,Month:w.Turn%12+1};for _,e:=range w.Empires{n:=0;for _,x:=range w.Planets{if x.OwnerID==e.ID{n++}};status:="sovereign";if e.Exile{status="exile"};if e.Eliminated{status="eliminated"};p.Rows=append(p.Rows,row{e.Name,n,status})};must(tpl.Execute(rw,p))});log.Println("AGame http://localhost:8080");log.Fatal(http.ListenAndServe(":8080",nil))}
func env(k,d string)string{if v:=os.Getenv(k);v!=""{return v};return d}
func must(err error){if err!=nil{log.Fatal(err)}}
