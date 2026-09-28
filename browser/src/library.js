import git from "isomorphic-git";
import { VFSFileSystem } from "@componentor/fs";
import { GitHubRemote } from "./jikko-github.js";

export class UniverseLibrary {
  constructor(fs, dir="/library") { this.fs=fs; this.dir=dir; this.remote=null; }
  static async open() {
    const fs=new VFSFileSystem({root:"/agame"}); await fs.init();
    await fs.promises.mkdir("/library",{recursive:true}); await fs.promises.mkdir("/sync",{recursive:true});
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
  async current() { return git.currentBranch({fs:this.fs,dir:this.dir}); }
  connectGitHub(url,token) { this.remote=GitHubRemote.fromURL(url,token); }
  async remoteUniverses() { if(!this.remote)return []; return (await this.remote.branches("universe/")).filter(x=>x.startsWith("universe/")); }
  async createUniverse({id,name,world}) {
    const ref=this.branchName(id,name);
    const branches=await git.listBranches({fs:this.fs,dir:this.dir});
    if(branches.includes(ref)) throw new Error("A universe with this id already exists");
    await git.branch({fs:this.fs,dir:this.dir,ref,checkout:true});
    await this.write("world.json",JSON.stringify(world,null,2));
    await this.write("universe.json",JSON.stringify({id,name,created:new Date().toISOString()},null,2));
    await this.commit("Turn 0 — universe created","new-universe"); return ref;
  }
  async load(ref) { await git.checkout({fs:this.fs,dir:this.dir,ref}); return JSON.parse(await this.read("world.json")); }
  async restore(ref) {
    if(!this.remote)throw new Error("GitHub is not connected");
    const local=await this.localUniverses(); if(local.includes(ref))return this.load(ref);
    const snap=await this.remote.readBranch(ref);
    await git.branch({fs:this.fs,dir:this.dir,ref,checkout:true});
    for(const [path,bytes] of Object.entries(snap.files)) await this.fs.promises.writeFile(this.dir+"/"+path,bytes);
    await this.commit("Restore "+ref+" from GitHub","restore");
    await this.setSyncHead(ref,snap.sha); return JSON.parse(new TextDecoder().decode(snap.files["world.json"]));
  }
  async saveTurn(world, cognition={}) {
    await this.write("world.json",JSON.stringify(world,null,2));
    await this.write("last-turn.json",JSON.stringify(cognition,null,2));
    const journal={turn:world.Turn,statements:cognition.statements||{},events:cognition.result?.events||[]};
    await this.write("jikko-turn.json",JSON.stringify(journal,null,2));
    return this.commit("Turn "+world.Turn,"turn");
  }
  async backupCurrent() {
    if(!this.remote)throw new Error("GitHub is not connected");
    const ref=await this.current(); if(!ref?.startsWith("universe/"))throw new Error("No universe checked out");
    const files=await this.snapshot(); const expected=await this.syncHead(ref);
    const remoteBranches=await this.remoteUniverses();
    if(remoteBranches.includes(ref)&&!expected)throw new Error("This universe already exists remotely. Restore it first to avoid overwriting another history.");
    const sha=await this.remote.writeBranch(ref,files,{message:"AGame "+ref+" backup",expectedHead:expected});
    await this.setSyncHead(ref,sha); return sha;
  }
  async snapshot() {
    const files={}; for(const name of await this.fs.promises.readdir(this.dir)){
      if(name===".git")continue; const p=this.dir+"/"+name, st=await this.fs.promises.stat(p);
      if(!st.isDirectory())files[name]=new Uint8Array(await this.fs.promises.readFile(p));
    } return files;
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
  async syncHead(ref){try{return new TextDecoder().decode(await this.fs.promises.readFile("/sync/"+encodeURIComponent(ref)))}catch{return null}}
  async setSyncHead(ref,sha){await this.fs.promises.writeFile("/sync/"+encodeURIComponent(ref),new TextEncoder().encode(sha))}
}
function slug(s){return String(s||"game").toLowerCase().replace(/[^a-z0-9]+/g,"-").replace(/^-|-$/g,"").slice(0,32)||"game"}
