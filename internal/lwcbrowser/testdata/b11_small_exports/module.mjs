import assert from 'node:assert/strict';
let input=''; for await (const chunk of process.stdin) input+=chunk;
const data=JSON.parse(input);
const mod=data.case==='graph' ? await import('../../../../lwcruntime/src/lightning/graphql.mjs') : await import('data:text/javascript;base64,'+Buffer.from(data.source).toString('base64'));
let actual;
if(data.case==='refresh') {
 actual={complete:mod.REFRESH_COMPLETE,error:mod.REFRESH_ERROR,withError:mod.REFRESH_COMPLETE_WITH_ERROR};
} else if(data.case==='mobile') {
 actual={}; for(const [key, f] of [['location',mod.getLocationService],['contacts',mod.getContactsService]]) {
  try { actual[key]={type:typeof f,available:f().isAvailable()}; } catch(e) { actual[key]={error:String(e)}; }
 }
} else {
 actual=[]; for(const x of [undefined,null,{}, {query:'mutation { nope }'}]) {
  try { const y=mod.executeMutation(x); const kind=y&&typeof y.then==='function'?'promise':'sync';
   try { const value=await y; actual.push({kind,value:value===undefined?'undefined':value}); }
   catch(e) {actual.push({kind,error:String(e),name:e?.name,message:e?.message});}
  } catch(e) {actual.push({kind:'throw',error:String(e),name:e?.name,message:e?.message});}
 }
}
assert.deepEqual(JSON.parse(JSON.stringify(actual)),JSON.parse(data.expected));
