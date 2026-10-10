// Execute the captured owned observer against product pages only.
// Native answers are never inputs; no Salesforce or external network access.
import fs from "node:fs";
import {createRequire} from "node:module";
const config=JSON.parse(fs.readFileSync(0,"utf8"));
const {chromium}=createRequire(import.meta.url)(config.playwrightModule);
const action=async (el,c)=>{
 const token=++window.v13Seq;
 const shape=(v,seen)=>{
  if(v===undefined)return {kind:'undefined'};
  if(v===null || typeof v!=='object')return v;
  if(v instanceof Date)return {kind:'date',value:v.toISOString()};
  seen=seen||new Set();if(seen.has(v))return {kind:'cycle'};seen.add(v);
  if(Array.isArray(v))return v.map(x=>shape(x,seen));
  const out={};Object.keys(v).sort().forEach(k=>{if(typeof v[k]!=='function')out[k]=shape(v[k],seen);});return out;
 };
 const eventShape=e=>e?{status:e.status,type:e.type,message:e.message,where:e.where,
  action:e.action,method:e.method,tidType:typeof e.tid}:null;
 const callback=function(result,event){
  window.v13Callbacks.push({invocation:token,argc:arguments.length,result:shape(result),event:eventShape(event)});
  if(c.callback==='throw'){try{throw new Error('V13_CALLBACK_THROW');}catch(e){window.v13Exceptions.push({invocation:token,name:e.name,message:e.message});}}
 };
 const before=window.v13Callbacks.length;
 let synchronous=false;
 try{
  if(c.mode==='remote-object'){
   const Model=window.V13Model && window.V13Model.Account;
   if(typeof Model!=='function')throw new Error('V13_MODEL_UNAVAILABLE');
   const model=new Model(c.operation==='set'?{}:c.value);
   const remoteCallback=function(error,result,event){
    const isList=Array.isArray(result);
    // Never inspect returned org record values. Id==null criteria are fixed,
    // but even a rejected/ignored query cannot leak private org data.
    window.v13Callbacks.push({invocation:token,argc:arguments.length,error:shape(error),
      result:c.operation.startsWith('retrieve')?{kind:typeof result,isArray:isList,count:isList?result.length:null}:shape(result),event:eventShape(event)});
   };
   if(c.operation==='construct')window.v13ModelValues.push({invocation:token,Name:shape(model.get('Name')),Phone:shape(model.get('Phone')),Id:shape(model.get('Id'))});
   else if(c.operation==='alias')window.v13ModelValues.push({invocation:token,alias:shape(model.get('nameAlias')),name:shape(model.get('Name'))});
   else if(c.operation==='get')window.v13ModelValues.push({invocation:token,value:shape(model.get(c.field))});
   else if(c.operation==='set'){const r=model.set(c.field,c.value);window.v13ModelValues.push({invocation:token,value:shape(model.get(c.field)),returnType:typeof r});}
   else if(c.operation==='retrieve' || c.operation==='retrieve-function')model.retrieve(c.operation==='retrieve-function'?()=>c.criteria:c.criteria,remoteCallback);
   else if(c.operation==='create')model.create(remoteCallback);
   else if(c.operation==='update')model.update(remoteCallback);
   else if(c.operation==='delete')model.del(remoteCallback);
   else throw new Error('V13_OPERATION_UNKNOWN');
   synchronous=['construct','alias','get','set'].includes(c.operation);
  }else{
   let cb=c.callback==='missing'?null:c.callback==='text'?'not-a-function':callback;
   const args=[window.v13Action].concat(c.args);
   if(c.callback!=='missing')args.push(cb);
   if(Object.prototype.hasOwnProperty.call(c,'options'))args.push(c.options);
   const manager=window.Visualforce && window.Visualforce.remoting && window.Visualforce.remoting.Manager;
   if(c.mode==='missing-method')manager.v13Missing.apply(manager,args);
   else if(c.mode==='direct'){
    const controller=window[window.v13Class];
    const directArgs=c.args.concat([cb]);if(Object.prototype.hasOwnProperty.call(c,'options'))directArgs.push(c.options);
    controller.call.apply(controller,directArgs);
   }else manager.invokeAction.apply(manager,args);
  }
 }catch(e){window.v13Exceptions.push({invocation:token,name:e.name,message:String(e.message)});return {observation:'synchronous-exception'};}
 if(synchronous)return {observation:'synchronous-model-operation'};
 const deadline=Date.now()+5000;
 while(Date.now()<deadline){
  if(window.v13Callbacks.length>before)return {observation:'callback'};
  await new Promise(r=>setTimeout(r,50));
 }
 return {observation:'no-callback-within-bound',boundMs:5000};
};
const snapshot=el=>({callbacks:window.v13Callbacks.slice().sort((a,b)=>a.invocation-b.invocation),exceptions:window.v13Exceptions.slice().sort((a,b)=>a.invocation-b.invocation),models:window.v13ModelValues.slice().sort((a,b)=>a.invocation-b.invocation),console:window.v13Console.filter(x=>/remot|remoteaction|V13|parameter|argument/i.test(x)).map(x=>x.replace(/https?:\/\/[^\s"'<>]+/g,'<URL>').replace(/(?:csrf|sessionId|access_token|sid)[=:][^\s,;]+/gi,'<REDACTED>'))});
const stable=value=>{
 if(Array.isArray(value))return value.map(stable);
 if(value && typeof value==='object')return Object.fromEntries(Object.keys(value).sort().map(k=>[k,stable(value[k])]));
 if(typeof value==='string')return value.replace(/FamilyV13[PC]\d{5}/g,'<owned>').replace(/Error ID: [^\s<]+/g,'Error ID: <native>');
 return value;
};
const browser=await chromium.launch({headless:true,timeout:15000,...(config.executablePath?{executablePath:config.executablePath}:{})});
try{
 const context=await browser.newContext();
 // Keep CI offline. Product HTML, generated scripts, and endpoint responses
 // are served unchanged by the Go product server.
 const origin=new URL(config.url).origin;
 await context.route('**/*',route=>new URL(route.request().url()).origin===origin?route.continue():route.abort());
 const page=await context.newPage();
 const values={};
 for(const spec of config.specs){
  await page.goto(config.url+spec.path,{waitUntil:'load',timeout:60000});
  const marker=page.locator('[data-case="'+spec.id+'"]');
  if(!(await marker.count())){
   values[spec.id]=stable(await page.evaluate(()=>({stage:'render',localRenderError:document.body.innerText,rootPresent:false})));
   continue;
  }
  const before=await marker.evaluate(snapshot);
  const outcomes=[];
  for(let step=0;step<spec.action.steps;step++)outcomes.push(await marker.evaluate(action,spec.action));
  values[spec.id]=stable({before,outcomes,after:await marker.evaluate(snapshot)});
 }
 process.stdout.write(JSON.stringify(values));
}finally{await browser.close();}
