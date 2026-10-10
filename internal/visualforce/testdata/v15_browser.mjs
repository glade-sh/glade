// Observe product-rendered pages; native answers are never browser inputs.
import fs from "node:fs";
import { createRequire } from "node:module";
const config = JSON.parse(fs.readFileSync(0, "utf8"));
const require = createRequire(import.meta.url);
const { chromium } = require(config.playwrightModule);
const browser = await chromium.launch({headless:true,timeout:15000,...(config.executablePath ? {executablePath:config.executablePath} : {})});
try {
 const context = await browser.newContext();
 const values = {};
 // Each spec observes its own page; the 500ms DOM-quiet wait dominates the cost,
 // so a small pool of concurrent pages overlaps those waits without changing
 // what any page observes.
 const observe = async spec => {
  const page = await context.newPage();
  try {
  await page.goto(config.url+spec.path,{waitUntil:"load",timeout:60000});
  if (spec.category==="exception") {
   values[spec.id]=await page.evaluate(()=>({ownedException:document.body.innerText.includes("V15 owned exception"),rootPresent:!!document.getElementById("v15-root")}));
   return;
  }
  if (!(await page.locator("#v15-root").count())) {
   values[spec.id]=await page.evaluate(()=>({localBrowserError:document.body.innerText,rootPresent:false}));
   return;
  }
  // A missing chart is an observed product difference, not a browser failure.
  // Wait for asynchronous initialization when present, then retain actual DOM.
  if (spec.chart) {
   await page.waitForFunction(()=>document.querySelector("#v15-root svg, #v15-root canvas"),null,{timeout:30000}).catch(()=>{});
  }
  if (spec.id==="dom_widget_script") {
   await page.waitForFunction(()=>window.v15OwnedAssetLoaded===true,null,{timeout:30000}).catch(()=>{});
  }
  await page.locator("#v15-root").evaluate(root=>new Promise((resolve,reject)=>{
   let quiet;
   const deadline=setTimeout(()=>{observer.disconnect();clearTimeout(quiet);reject(Error("owned DOM did not settle"));},30000);
   const settled=()=>{observer.disconnect();clearTimeout(deadline);resolve();};
   const arm=()=>{clearTimeout(quiet);quiet=setTimeout(settled,500);};
   const observer=new MutationObserver(arm);
   observer.observe(root,{subtree:true,childList:true,attributes:true,characterData:true});arm();
  }));
  const snapshot=()=>page.evaluate(()=>{
   const root=document.getElementById("v15-root");
   const stable=v=>v.replace(/https?:\/\/[^/\s"']+/g,"<HOST>").replace(/\/resource\/\d+\//g,"/resource/<VERSION>/").replace(/j_id\d+/g,"<AUTO>").replace(/FamilyV15Asset\d+/g,"FamilyV15Asset<API>").replace(/ext-gen\d+/g,"<EXT-AUTO>");
   const tree=node=>{
    if(node.nodeType===3)return {text:node.textContent};
    if(node.nodeType!==1 || ["SCRIPT","STYLE"].includes(node.tagName))return null;
    const attrs=Object.fromEntries([...node.attributes].filter(a=>!/^data-(ext|react)/.test(a.name)).map(a=>[a.name,stable(a.value)]).sort());
    const css=getComputedStyle(node);
    const value={tag:node.tagName.toLowerCase(),attrs,css:{display:css.display,color:css.color,paddingLeft:css.paddingLeft},children:[...node.childNodes].map(tree).filter(Boolean)};
    if(node.tagName==="IMG")value.image={complete:node.complete,width:node.naturalWidth,height:node.naturalHeight};
    if(node.tagName==="CANVAS")value.canvas={width:node.width,height:node.height};
    return value;
   };
   const links=[...document.querySelectorAll('link[rel="stylesheet"]')].map(n=>n.getAttribute("href")||"");
   return {tree:tree(root),ownedAssetLoaded:window.v15OwnedAssetLoaded===true,
    styles:{ownedCss:links.some(v=>/FamilyV15Asset\d+Css/.test(v)),ownedZip:links.some(v=>/FamilyV15Asset\d+Zip/.test(v)),standard:links.some(v=>/\/sCSS\/|\/styles\//.test(v)),slds:links.some(v=>/slds/i.test(v))},
    headerPresent:!!document.querySelector("#AppBodyHeader, .bPageHeader, #tabNavigation")};
  });
  const before=await snapshot();
  if(spec.click){await page.locator('[id="target"], [id$=":target"]').first().click();values[spec.id]={before,after:await snapshot()};}
  else values[spec.id]=before;
  } finally {
   await page.close();
  }
 };
 const pending = [...config.specs];
 const workers = Array.from({length:Math.min(config.concurrency||4,pending.length)},async()=>{
  for (let spec=pending.shift();spec;spec=pending.shift()) await observe(spec);
 });
 await Promise.all(workers);
 // Completion order varies across pages; serialize in the original spec order.
 const orderedValues = Object.fromEntries(config.specs.map(spec=>[spec.id,values[spec.id]]));
 process.stdout.write(JSON.stringify(orderedValues));
} finally {
 await browser.close();
}
