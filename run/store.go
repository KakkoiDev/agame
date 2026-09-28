package run

import("bufio";"encoding/json";"os";"path/filepath";"github.com/KakkoiDev/agame/world")
type Store struct{Dir string}
func (s Store) Save(w *world.World)error{if err:=os.MkdirAll(s.Dir,0755);err!=nil{return err};b,err:=json.MarshalIndent(w,"","  ");if err!=nil{return err};return os.WriteFile(filepath.Join(s.Dir,"world.json"),b,0644)}
func (s Store) Load()(*world.World,error){b,err:=os.ReadFile(filepath.Join(s.Dir,"world.json"));if err!=nil{return nil,err};var w world.World;err=json.Unmarshal(b,&w);return &w,err}
func (s Store) Append(events []world.Event)error{if err:=os.MkdirAll(s.Dir,0755);err!=nil{return err};f,err:=os.OpenFile(filepath.Join(s.Dir,"events.jsonl"),os.O_CREATE|os.O_APPEND|os.O_WRONLY,0644);if err!=nil{return err};defer f.Close();bw:=bufio.NewWriter(f);defer bw.Flush();enc:=json.NewEncoder(bw);for _,e:=range events{if err:=enc.Encode(e);err!=nil{return err}};return nil}
