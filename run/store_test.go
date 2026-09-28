package run

import("testing";"github.com/KakkoiDev/agame/world")
func TestRoundTrip(t *testing.T){d:=t.TempDir();s:=Store{Dir:d};w,_:=world.Generate(9,[]string{"A","B","C","D","E","F","G","H"});if err:=s.Save(w);err!=nil{t.Fatal(err)};x,err:=s.Load();if err!=nil||x.Seed!=9||len(x.Planets)!=128{t.Fatal("roundtrip")}}
