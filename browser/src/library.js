import git from "isomorphic-git";
import http from "isomorphic-git/http/web";
import { VFSFileSystem } from "@componentor/fs";

// AGame assigns universe semantics to branches. This adapter intentionally
// mirrors Jikko BrowserGit's provider-neutral Git/OPFS contract; AGame never
// stores credentials in universe files or remote URLs.
export class UniverseLibrary {
  constructor(fs, dir="/library") { this.fs=fs; this.dir=dir; }
  static async open() {
    const fs=new VFSFileSystem({root:"/agame"});
    await fs.init(); await fs.promises.mkdir("/library",{recursive:true});
    const x=new UniverseLibrary(fs);
    try { await fs.promises.stat("/library/.git"); } catch {
      await git.init({fs,dir:"/library",defaultBranch:"main"});
      await fs.promises.writeFile("/library/library.json",new TextEncoder().encode('{"format":1}'));
      await git.add({fs,dir:"/library",filepath:"library.json"});
      await git.commit({fs,dir:"/library",message:"Initialize AGame library",author:{name:"AGame Browser",email:"browser@agame.local"}});
    }
    return x;
  }
  branchName(id,name) { return "universe/"+id+"-"+slug(name); }
  async localUniverses() { return (await git.listBranches({fs:this.fs,dir:this.dir})).filter(x=>x.startsWith("universe/")); }
  async remoteUniverses(remote="origin") {
    try { return (await git.listBranches({fs:this.fs,dir:this.dir,remote})).filter(x=>x.startsWith("universe/")); } catch { return []; }
  }
  async current() { return git.currentBranch({fs:this.fs,dir:this.dir}); }
  async createUniverse({id,name,world}) {
    const ref=this.branchName(id,name);
    await git.branch({fs:this.fs,dir:this.dir,ref,checkout:true});
    await this.write("world.json",JSON.stringify(world,null,2));
    await this.write("universe.json",JSON.stringify({id,name,created:new Date().toISOString()},null,2));
    await this.commit("Turn 0 — universe created","new-universe");
    return ref;
  }
  async load(ref) { await git.checkout({fs:this.fs,dir:this.dir,ref}); return JSON.parse(await this.read("world.json")); }
  async saveTurn(world) {
    await this.write("world.json",JSON.stringify(world,null,2));
    return this.commit("Turn "+world.Turn,"turn");
  }
  async write(path,data) { await this.fs.promises.writeFile(this.dir+"/"+path,new TextEncoder().encode(data)); }
  async read(path) { return new TextDecoder().decode(await this.fs.promises.readFile(this.dir+"/"+path)); }
  async commit(message,operation) {
    for(const [filepath,head,workdir] of await git.statusMatrix({fs:this.fs,dir:this.dir})) {
      if(workdir===0) await git.remove({fs:this.fs,dir:this.dir,filepath});
      else if(head!==workdir) await git.add({fs:this.fs,dir:this.dir,filepath});
    }
    return git.commit({fs:this.fs,dir:this.dir,message:message+"\n\nJikko-Actor: agame\nJikko-Operation: "+operation,author:{name:"AGame Browser",email:"browser@agame.local"}});
  }
  async configureRemote(url) {
    if(/[^:]\/\/[^/]*@/.test(url)) throw new Error("Do not put credentials in the remote URL");
    try { await git.deleteRemote({fs:this.fs,dir:this.dir,remote:"origin"}); } catch {}
    await git.addRemote({fs:this.fs,dir:this.dir,remote:"origin",url});
  }
  async fetchCatalog(auth) { return git.fetch({fs:this.fs,http,dir:this.dir,remote:"origin",singleBranch:false,onAuth:()=>auth}); }
  async fetchUniverse(ref,auth) { return git.fetch({fs:this.fs,http,dir:this.dir,remote:"origin",ref,singleBranch:true,onAuth:()=>auth}); }
  async pushCurrent(auth) {
    const ref=await this.current(); if(!ref) throw new Error("No universe checked out");
    return git.push({fs:this.fs,http,dir:this.dir,remote:"origin",ref,remoteRef:ref,onAuth:()=>auth});
  }
}
function slug(s){return String(s||"game").toLowerCase().replace(/[^a-z0-9]+/g,"-").replace(/^-|-$/g,"").slice(0,32)||"game"}
