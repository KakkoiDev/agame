import {UniverseLibrary} from "./library.js";

const $=s=>document.querySelector(s);
let lib, currentRef="", world=null;

async function boot(){
  while(!globalThis.AGameWASM) await new Promise(r=>setTimeout(r,20));
  lib=await UniverseLibrary.open();
  await refresh();
  $("#new-game").addEventListener("submit",newGame);
  $("#advance").addEventListener("click",advance);
  $("#sync").addEventListener("click",sync);
  $("#connect").addEventListener("submit",connect);
  navigator.serviceWorker?.register("./sw.js").catch(()=>{});
}
async function refresh(){
  const local=await lib.localUniverses(); let remote=[]; try{remote=await lib.remoteUniverses()}catch{}
  const all=[...new Set([...local,...remote])];
  $("#games").innerHTML=all.length?all.map(r=>{const here=local.includes(r);return `<button class="game" data-ref="${esc(r)}" data-local="${here}">${esc(r.replace("universe/",""))} · ${here?"local":"GitHub"}</button>`}).join(""):"<p>No universes yet.</p>";
  document.querySelectorAll(".game").forEach(b=>b.addEventListener("click",()=>b.dataset.local==="true"?openGame(b.dataset.ref):restoreGame(b.dataset.ref)));
}
async function newGame(e){
  e.preventDefault(); const name=$("#name").value.trim()||"New universe"; const seed=Number($("#seed").value)||1;
  const r=AGameWASM.newUniverse(seed); if(!r.ok) return status(r.error);
  const id=crypto.randomUUID().slice(0,8); world=JSON.parse(r.json); currentRef=await lib.createUniverse({id,name,world});
  await refresh(); render(); status("Created locally. Ready offline.");
}
async function openGame(ref){ world=await lib.load(ref); currentRef=ref; render(); }
async function restoreGame(ref){status("Downloading universe…");world=await lib.restore(ref);currentRef=ref;await refresh();render();status("Universe restored locally and ready offline.");}
async function advance(){
  if(!world)return; const r=AGameWASM.advanceTurn(JSON.stringify(world),"{}"); if(!r.ok)return status(r.error);
  const out=JSON.parse(r.json); world=out.world; await lib.saveTurn(world); render(); status("Turn committed locally.");
  if(sessionStorage.getItem("agame-sync")==="1") sync().catch(()=>{});
}
async function connect(e){
  e.preventDefault(); const url=$("#remote").value.trim(), token=$("#token").value.trim(); if(!url||!token)return;
  lib.connectGitHub(url,token); sessionStorage.setItem("agame-sync","1");
  status("GitHub backup connected for this browser session."); await refresh(); if(currentRef) await sync();
}
async function sync(){
  if(!lib.remote)return status("Connect GitHub first.");
  status("Syncing…");
  try{await lib.backupCurrent();status("Backed up to GitHub.");
  catch(e){status("Local save is safe; backup pending: "+e.message)}
}
function render(){
  $("#play").hidden=!world;if(!world)return;
  const s=JSON.parse(AGameWASM.summary(JSON.stringify(world)).json);
  $("#turn").textContent=`Turn ${s.turn} · seed ${s.seed}`;
  $("#empires").innerHTML=s.empires.map(e=>`<tr><td>${esc(e.Name||e.name)}</td><td>${e.Planets??e.planets}</td><td>${(e.Eliminated??e.eliminated)?"eliminated":(e.Exile??e.exile)?"exile":"sovereign"}</td></tr>`).join("");
}
function status(x){$("#status").textContent=x}
function esc(x){return String(x).replace(/[&<>"]/g,c=>({"&":"&amp;","<":"&lt;",">":"&gt;",'"':"&quot;"}[c]))}
boot();
