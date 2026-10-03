import {test,expect} from "@playwright/test";

async function ready(page){
  const errors=[]; page.on("pageerror",e=>errors.push(e.message));
  await page.goto("/");
  await page.waitForFunction(()=>["Ready","Startup failed"].includes(document.querySelector("#startup-stage")?.textContent),{timeout:20000});
  const stage=await page.locator("#startup-stage").textContent();
  if(stage!=="Ready")throw new Error("startup: "+await page.locator("#startup-detail").textContent()+" errors="+errors.join(" | "));
  return errors;
}
async function create(page,name,seed=1){
  await page.locator("#name").fill(name); await page.locator("#seed").fill(String(seed));
  await page.getByRole("button",{name:"New game"}).click();
  await expect(page.locator("#status")).toContainText("Universe created",{timeout:10000});
  await expect(page.locator("#play")).toBeVisible();
}

test("creates and advances a universe in the real browser",async({page})=>{
  const errors=await ready(page); await create(page,"Smoke",1);
  await expect(page.locator("#turn")).toContainText("Turn 0");
  await page.getByRole("button",{name:"Advance 1 month"}).click();
  await expect(page.locator("#turn")).toContainText("Turn 1",{timeout:10000});
  expect(errors).toEqual([]);
});

test("New game always creates a distinct universe",async({page})=>{
  await ready(page); await create(page,"Same Name",7);
  const first=await page.locator(".game").first().getAttribute("data-ref");
  await create(page,"Same Name",7);
  const refs=await page.locator(".game").evaluateAll(xs=>xs.map(x=>x.dataset.ref));
  expect(refs.length).toBe(2); expect(new Set(refs).size).toBe(2); expect(refs).toContain(first);
  await expect(page.locator("#turn")).toContainText("Turn 0");
});

test("two universes keep independent turns and world seeds",async({page})=>{
  await ready(page); await create(page,"Alpha",11);
  await page.getByRole("button",{name:"Advance 1 year"}).click();
  await expect(page.locator("#turn")).toContainText("Turn 12",{timeout:20000});
  const alpha=await page.locator(".game").filter({hasText:"alpha"}).getAttribute("data-ref");
  await create(page,"Beta",22); await expect(page.locator("#turn")).toContainText("Turn 0");
  const beta=await page.locator(".game").filter({hasText:"beta"}).getAttribute("data-ref");
  expect(alpha).not.toBe(beta);
  await page.locator(`.game[data-ref="${alpha}"]`).click(); await expect(page.locator("#turn")).toContainText("Turn 12");
  await page.locator(`.game[data-ref="${beta}"]`).click(); await expect(page.locator("#turn")).toContainText("Turn 0");
});

test("save survives a page reload and continues",async({page})=>{
  await ready(page); await create(page,"Persistent",33);
  await page.getByRole("button",{name:"Advance 1 month"}).click(); await expect(page.locator("#turn")).toContainText("Turn 1");
  const ref=await page.locator(".game").filter({hasText:"persistent"}).getAttribute("data-ref");
  await page.reload(); await page.waitForFunction(()=>document.querySelector("#startup-stage")?.textContent==="Ready",{timeout:20000});
  await page.locator(`.game[data-ref="${ref}"]`).click(); await expect(page.locator("#turn")).toContainText("Turn 1");
  await page.getByRole("button",{name:"Advance 1 month"}).click(); await expect(page.locator("#turn")).toContainText("Turn 2");
});

test("57 turns visibly evolve the strategic state",async({page})=>{
  await ready(page); await create(page,"Long Run",57);
  for(let i=0;i<4;i++)await page.getByRole("button",{name:"Advance 1 year"}).click();
  for(let i=0;i<9;i++)await page.getByRole("button",{name:"Advance 1 month"}).click();
  await expect(page.locator("#turn")).toContainText("Turn 57",{timeout:30000});
  // columns: rank, empire, planets, fleets, tech, …
  const rows=await page.locator("#empires tr").evaluateAll(rs=>rs.map(r=>Array.from(r.cells).map(c=>c.textContent)));
  expect(rows.some(r=>Number(r[2])>1)).toBeTruthy();
  expect(rows.some(r=>Number(r[3])>0)).toBeTruthy();
  expect(rows.some(r=>Number(r[4])>0)).toBeTruthy();
  await page.locator("summary").filter({hasText:"Last turn decisions"}).click();
  await expect(page.locator("#decisions")).not.toHaveText("");
});
