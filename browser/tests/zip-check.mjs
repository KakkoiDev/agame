// Node self-check for src/zip.js (run by `npm run check`).
import {zip,unzip,crc32} from "../src/zip.js";
import assert from "node:assert/strict";
assert.equal(crc32(new TextEncoder().encode("123456789")),0xcbf43926);
const files={"world.json":JSON.stringify({Turn:3,Seed:7}),"universe.json":'{"name":"Ünïcode"}',"empty.txt":new Uint8Array()};
const a=zip(files),b=zip({"empty.txt":files["empty.txt"],"universe.json":files["universe.json"],"world.json":files["world.json"]});
assert.deepEqual(a,b,"export must not depend on insertion order");
const back=unzip(a);
assert.deepEqual(Object.keys(back).sort(),["empty.txt","universe.json","world.json"]);
assert.equal(new TextDecoder().decode(back["universe.json"]),files["universe.json"]);
assert.equal(JSON.parse(new TextDecoder().decode(back["world.json"])).Turn,3);
const bad=a.slice(),at=Buffer.from(a).indexOf("Turn");bad[at]^=0xff;
assert.throws(()=>unzip(bad),/checksum/);
assert.throws(()=>unzip(new Uint8Array(10)),/not a ZIP/);
assert.throws(()=>unzip(zip({"../evil":"x"})),/unsafe/);
console.log("zip ok");
