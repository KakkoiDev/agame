// Node check of universe ZIP export/import through the real library code,
// using isomorphic-git on the local filesystem instead of OPFS.
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import assert from "node:assert/strict";
import git from "isomorphic-git";
import {UniverseLibrary} from "../src/library.js";
import {unzip} from "../src/zip.js";
const dir=fs.mkdtempSync(path.join(os.tmpdir(),"agame-lib-"));
try{
  await git.init({fs,dir,defaultBranch:"main"});
  fs.writeFileSync(path.join(dir,"library.json"),'{"format":1}');
  await git.add({fs,dir,filepath:"library.json"});
  await git.commit({fs,dir,message:"init",author:{name:"t",email:"t@t"}});
  const lib=new UniverseLibrary(fs,dir);
  const world={Turn:0,Seed:5,Empires:{e00:{ID:"e00"}}};
  const ref=await lib.createUniverse({id:"abc",name:"First",world});
  await lib.saveTurn({...world,Turn:1},{statements:{e00:"hi"},result:{events:[]}});
  const {ref:exported,bytes}=await lib.exportCurrent();
  assert.equal(exported,ref);
  const files=unzip(bytes);
  assert.ok(files["world.json"]&&files["universe.json"]&&files["last-turn.json"]);
  assert.ok(!Object.keys(files).some(f=>f.startsWith(".git")),"Git internals must not be exported");
  const imp=await lib.importUniverse(bytes,"def");
  assert.equal(imp.world.Turn,1);
  assert.match(imp.ref,/^universe\/def-first-imported$/);
  const meta=JSON.parse(await lib.read("universe.json"));
  assert.equal(meta.id,"def");assert.equal(meta.source,"abc");
  assert.deepEqual((await lib.localUniverses()).sort(),[ref,imp.ref].sort());
  assert.equal((await lib.load(ref)).Turn,1,"the original universe is untouched");
  await assert.rejects(lib.importUniverse(bytes,"def"),/already exists/);
  await assert.rejects(lib.importUniverse(new Uint8Array(30),"x"),/ZIP/);
  console.log("library ok");
}finally{fs.rmSync(dir,{recursive:true,force:true})}
