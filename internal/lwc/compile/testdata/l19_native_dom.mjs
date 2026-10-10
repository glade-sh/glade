// Owned native record forms projections, unchanged from owned capture exports.
// Local connection plumbing supplies a page; no native answers enter this module.
import { AsyncLocalStorage } from "node:async_hooks";
const rpcState = new AsyncLocalStorage();
const context = {
  newCDPSession: async target => {
    const cdp = await rpcState.getStore().page.context().newCDPSession(target);
    const send = cdp.send.bind(cdp);
    let idleTurn;
    cdp.send = async (method, params) => {
      const result = await send(method, params);
      if (method === "Debugger.pause") {
        // The native renderer was busy. A local renderer can be idle, so give
        // its pending pause one side-effect-free JS turn. The unchanged native
        // observer still requires a real Debugger.paused event before reading.
        // This evaluation may finish only after resume; never await it here.
        idleTurn = send("Runtime.evaluate", { expression: "void 0", returnByValue: true })
          .then(() => ({}), error => ({ error }));
      } else if (method === "Debugger.resume" && idleTurn) {
        const completed = await idleTurn;
        idleTurn = undefined;
        if (completed.error) throw completed.error;
      }
      return result;
    };
    return cdp;
  },
};
const safe=s=>String(s||'').replace(/https?:\/\/[^\s"'<>]+/gi,'<URL>')
 .replace(/\bBearer\s+\S+/gi,'Bearer <REDACTED>')
 .replace(/\b(?:access_?token|refresh_?token|client_?secret|sessionId|sfdxAuthUrl|password|authorization|sid|otp|frontdoor_uri)\s*["']?\s*[:=]\s*(?:"[^"]*"|'[^']*'|[^\s,;<>]+)/gi,'<REDACTED>')
 .replace(/\b00D[A-Za-z0-9]{12}(?:[A-Za-z0-9]{3})?![A-Za-z0-9._-]+/g,'<REDACTED>');

export const observeDOM = root=>{
 const nodes=[];
 function visit(el){
  for(const child of el.children || []){nodes.push(child);visit(child);if(child.shadowRoot)visit(child.shadowRoot);}
 }
 visit(root);
 const seen=new Set();
 const fieldName=n=>typeof n.fieldName==='string'?n.fieldName:n.fieldName&&n.fieldName.fieldApiName;
 const fields=nodes.filter(n=>['LIGHTNING-INPUT-FIELD','LIGHTNING-OUTPUT-FIELD'].includes(n.tagName)).map(n=>({component:n.tagName.toLowerCase(),field:fieldName(n),text:n.innerText||n.textContent||''}));
 const supported=new Set(['Name','LastName','Phone','Website','AnnualRevenue','NumberOfEmployees','Description','Industry','Email','Birthdate','AccountId','IsDeleted']);
 const ownedFields=nodes.filter(n=>['LIGHTNING-INPUT-FIELD','LIGHTNING-OUTPUT-FIELD'].includes(n.tagName)&&supported.has(fieldName(n)));
 const projected=new Set();for(const field of ownedFields){projected.add(field);const local=[];function walk(n){for(const c of n.children||[]){projected.add(c);walk(c);if(c.shadowRoot)walk(c.shadowRoot);}}walk(field);if(field.shadowRoot)walk(field.shadowRoot);}
 const controls=nodes.filter(n=>['INPUT','TEXTAREA','SELECT'].includes(n.tagName)).filter(n=>{if(root.dataset.component!=='picker'&&!projected.has(n))return false;if(seen.has(n))return false;seen.add(n);return true;}).map(n=>({tag:n.tagName.toLowerCase(),type:n.type,value:n.value,checked:n.checked,disabled:n.disabled,required:n.required,readOnly:n.readOnly,placeholder:n.placeholder,invalid:n.getAttribute('aria-invalid')}));
 const buttons=nodes.filter(n=>n.tagName==='BUTTON').map(n=>({text:n.innerText||n.textContent||'',title:n.title,disabled:n.disabled}));
 const errors=nodes.filter(n=>n.getAttribute('role')==='alert'||(n.classList && n.classList.contains('slds-form-element__help'))).map(n=>n.innerText||n.textContent||'');
 return {fields:fields.filter(f=>supported.has(f.field)),controls,buttons,errors};
};

export const actDOM = async(root,arg)=>{
 const sleep=ms=>new Promise(r=>setTimeout(r,ms));
 function all(){const result=[];function visit(el){for(const child of el.children||[]){result.push(child);visit(child);if(child.shadowRoot)visit(child.shadowRoot);}}visit(root);return result;}
 const text=n=>n.innerText||n.textContent||'';
 const outcome={operation:arg.operation};
 const setInput=(input,value)=>{
  input.focus();const proto=input.tagName==='TEXTAREA'?HTMLTextAreaElement.prototype:HTMLInputElement.prototype;
  Object.getOwnPropertyDescriptor(proto,'value').set.call(input,value);
  input.dispatchEvent(new Event('input',{bubbles:true,composed:true}));
  input.dispatchEvent(new Event('change',{bubbles:true,composed:true}));
 };
 if(arg.operation.startsWith('search-')){
  let input;
  for(let i=0;i<30;i++){input=all().find(n=>n.tagName==='INPUT' && ['search','text'].includes(n.type));if(input)break;await sleep(100);}
  if(!input)return {...outcome,noSearchInputWithinMs:3000};
  outcome.inputDisabled=input.disabled;
  if(input.disabled)return outcome;
  const query=arg.operation==='search-phone'?'415-555-0100':arg.operation==='search-no-match'?arg.noMatch:arg.name;
  setInput(input,query);outcome.query=query;
  let option;
  for(let i=0;i<80;i++){
   option=all().find(n=>(n.getAttribute('role')==='option'||n.getAttribute('role')==='presentation'&&n.tagName==='LI')&&text(n).includes(arg.name));
   if(option && arg.operation!=='search-no-match')break;await sleep(100);
  }
  if(arg.operation==='search-no-match')return {...outcome,ownedSuggestionPresent:!!option,observedWithinMs:8000};
  if(!option)return {...outcome,noOwnedSuggestionWithinMs:8000};
  option.click();outcome.selected=true;await sleep(1000);
  if(arg.operation==='search-clear'){
   const clear=all().find(n=>n.tagName==='BUTTON'&&/remove|clear/i.test(n.title+' '+n.getAttribute('aria-label')));
   if(clear){clear.click();outcome.cleared=true;}else outcome.noClearControl=true;
  }
 } else if(arg.operation==='field-change'){
  const input=all().find(n=>n.tagName==='INPUT'&&!n.disabled&&!n.readOnly);
  if(input){setInput(input,'NATIVE CHANGED');input.blur();outcome.changed=true;}else outcome.noEditableInput=true;
 } else if(arg.operation==='reset'){
  const input=all().find(n=>n.tagName==='INPUT'&&!n.disabled&&!n.readOnly);
  if(input){setInput(input,'NATIVE CHANGED BEFORE RESET');input.blur();outcome.changedBeforeReset=true;}else outcome.noEditableInput=true;
 } else if(arg.operation==='cancel'||arg.operation==='inline-edit'){
  const button=all().find(n=>n.tagName==='BUTTON'&&(arg.operation==='cancel'?/cancel/i.test(text(n)):/edit/i.test(n.title+' '+n.getAttribute('aria-label'))));
  if(button){button.click();outcome.clicked=true;}else outcome.noNativeControl=true;
 }
 return outcome;
};

export const mountDOM = (root,state)=>{
 const input=root.querySelector('[data-case-state]');
 const start=root.querySelector('[data-start]');
 if(!input || !start)throw new Error('L19 batch shell unavailable');
 input.value=JSON.stringify(state);start.click();
 return {id:state.c__case,started:true};
};


const l19Delay=ms=>new Promise(resolve=>setTimeout(resolve,ms));
function l19Exception(error,operation){
 return {operation,exception_type:safe(error?.name||'Error'),
  message:safe(error?.message||String(error)).replace(/\b(?:token|session|cookie)\s*["']?\s*[:=]\s*(?:"[^"]*"|'[^']*'|[^\s,;<>]+)/gi,'<REDACTED>').slice(0,8192)};
}
async function l19Deadline(work,ms,reason){
 const state=rpcState.getStore();
 const names={L19_NATIVE_PAUSE_FAILED:'Debugger.pause',L19_NATIVE_PAUSE_UNCONFIRMED:'Debugger.paused event',
  L19_NATIVE_DOM_UNAVAILABLE:'DOM.getDocument',L19_NATIVE_RESUME_FAILED:'Debugger.resume',
  L19_NATIVE_INSPECTOR_UNAVAILABLE:'Debugger.enable',L19_NATIVE_MOUNT_UNCONFIRMED:'mount form',
  L19_NATIVE_SWITCH_NODE_UNAVAILABLE:'DOM.resolveNode',L19_NATIVE_SWITCH_UNCONFIRMED:'Runtime.callFunctionOn switch',
  L19_NATIVE_INSPECTOR_DETACH_FAILED:'CDPSession.detach'};
 const operation=names[reason]||(reason==='CDP_CREATE_SESSION'?'BrowserContext.newCDPSession':reason),started=process.hrtime.bigint();
 let outcome='returned',failure;
 let timer;
 try{return await Promise.race([typeof work==='function'?Promise.resolve().then(work):work,new Promise((_,reject)=>{
  timer=setTimeout(()=>reject(new Error(reason)),ms);
 })]);}catch(error){
  outcome='raised';failure=l19Exception(error,operation);
  if(!error.l19Operation)error.l19Operation=operation;
  throw error;
 }finally{
  clearTimeout(timer);
  (state.l19Trace||(state.l19Trace=[])).push({operation,outcome,
   elapsed_ms:Number(process.hrtime.bigint()-started)/1000000,...(failure?{failure}:{})});
 }
}
function l19Children(node){
 return [...(node.children||[]),...(node.shadowRoots||[]),...(node.contentDocument?[node.contentDocument]:[])];
}
function l19Attributes(node){
 const attrs={};for(let i=0;i<(node.attributes||[]).length;i+=2)attrs[node.attributes[i]]=node.attributes[i+1];
 return attrs;
}
function l19Find(node,predicate){
 if(predicate(node))return node;
 for(const child of l19Children(node)){const found=l19Find(child,predicate);if(found)return found;}
 return null;
}
function l19OwnedHost(root){
 const hosts=[];
 const visit=node=>{if(Object.hasOwn(l19Attributes(node),'data-l19-host'))hosts.push(node);for(const child of l19Children(node))visit(child);};
 visit(root);
 if(hosts.length!==1)throw new Error('L19_NATIVE_HOST_NOT_UNIQUE');
 const host=hosts[0];
 if(host.nodeName!=='SECTION'||!Number.isInteger(host.backendNodeId)||host.backendNodeId<=0||
  !l19Find(host,node=>Object.hasOwn(l19Attributes(node),'data-case-state'))||
  !l19Find(host,node=>Object.hasOwn(l19Attributes(node),'data-start')))
  throw new Error('L19_NATIVE_HOST_IDENTITY_UNAVAILABLE');
 return host.backendNodeId;
}
function l19Witness(root,id,owned){
 // The mounted fixture may replace its outer SECTION. The pre-mount ID
 // proves initial access, not identity across native rendering. Resolve the
 // unique structural fixture again from this fresh, paused native document.
 // No cached tree, JS getter, fabricated marker, or timeout is an answer.
 const current=l19OwnedHost(root);
 const host=l19Find(root,node=>node.backendNodeId===current);
 const native=l19Find(host,node=>Object.hasOwn(l19Attributes(node),'data-native'));
 const dto=l19Find(host,node=>Object.hasOwn(l19Attributes(node),'data-result'));
 const text=node=>node.nodeType===3?node.nodeValue||'':l19Children(node).map(text).join('');
 let controller=null;
 if(dto){const raw=text(dto);if(raw){try{controller=JSON.parse(raw);}catch(_){controller={raw:safe(raw)};}}}
 if(controller?.id && controller.id!==id)throw new Error('L19_NATIVE_ROW_MARKER_MISMATCH');
 let count=0,truncated=false;
 const project=(node,inError=false)=>{
  if(++count>5000){truncated=true;return {truncated:true};}
  if(node.nodeType===3){
   const raw=node.nodeValue||'';
   if(!inError)return {textPresent:Boolean(raw.trim())};
   if(raw.length>4096)truncated=true;return {text:safe(raw.slice(0,4096))};
  }
  const raw=l19Attributes(node);
  inError=inError || raw.role==='alert' || /(?:slds-form-element__help|errorMsg|errorMessage)/.test(raw.class||'');
  const attributes={};
  for(const [key,value] of Object.entries(raw)){
   // Layouts may expose volatile Created/Modified fields and generated IDs.
   // Preserve actual control state, not unrelated field values or ID joins.
   if(key==='value')attributes.valuePresent=Boolean(value);
   else if(/^(type|role|name|checked|disabled|required|field-name|mode|columns|layout-type|aria-(?:label|invalid|disabled|required|readonly|expanded|selected|busy|live))$/.test(key))attributes[key]=safe(value);
  }
  return {tag:node.nodeName,attributes,children:l19Children(node).map(child=>project(child,inError))};
 };
 const mask=l19Find(root,node=>l19Attributes(node).id==='auraErrorMask');
 const form=native && l19Find(native,node=>node.nodeName==='LIGHTNING-RECORD-FORM');
 return {hostTag:host.nodeName,hostIdentityChanged:current!==owned,hostResolution:'unique post-window native fixture',nativeContainerPresent:Boolean(native),recordFormPresent:Boolean(form),tree:native?project(native):null,
  controller,auraMask:mask?{attributes:l19Attributes(mask),text:safe(text(mask))}:null,truncated};
}
async function l19NativeWindow(cdp,id,owned,switchRecord=false){
 // A Node timer still fires if native rendering or a patched DOM getter spins.
 // Pausing the renderer lets the inspector read real nodes without evaluating
 // page JS. No timeout, transport error, or cached snapshot is an answer.
 const state=rpcState.getStore(),windowStarted=process.hrtime.bigint();
 await l19Delay(8000);
 (state.l19Trace||(state.l19Trace=[])).push({operation:'settle window',outcome:'returned',
  requested_ms:8000,elapsed_ms:Number(process.hrtime.bigint()-windowStarted)/1000000});
 let onPaused,primaryError,observationComplete=false;
 const paused=new Promise(resolve=>{onPaused=resolve;cdp.once('Debugger.paused',onPaused);});
 try{
  await l19Deadline(()=>cdp.send('Debugger.pause'),2000,'L19_NATIVE_PAUSE_FAILED');
  await l19Deadline(paused,2000,'L19_NATIVE_PAUSE_UNCONFIRMED');
  const doc=await l19Deadline(()=>cdp.send('DOM.getDocument',{depth:-1,pierce:true}),2000,'L19_NATIVE_DOM_UNAVAILABLE');
  try{state.l19LastSnapshot={status:'captured',value:l19Witness(doc.root,id,owned)};}
  catch(error){state.l19LastSnapshot={status:'owned-state-unavailable',rootTag:doc.root.nodeName,failure:l19Exception(error,'project native owned DOM')};error.l19Operation='project native owned DOM';throw error;}
  const witness=state.l19LastSnapshot.value;
  let operationResult;
  if(switchRecord){
   const host=l19Find(doc.root,node=>node.backendNodeId===l19OwnedHost(doc.root));
   const button=l19Find(host,node=>Object.hasOwn(l19Attributes(node),'data-run'));
   if(!button)throw new Error('L19_NATIVE_SWITCH_BUTTON_MISSING');
   const remote=await l19Deadline(()=>cdp.send('DOM.resolveNode',{nodeId:button.nodeId}),1000,'L19_NATIVE_SWITCH_NODE_UNAVAILABLE');
   if(!remote.object?.objectId)throw new Error('L19_NATIVE_SWITCH_NODE_UNAVAILABLE');
   // Invoke the real button while the renderer is interrupted: queuing a new
   // Playwright evaluation behind the stalled native task would stall again.
   const called=await l19Deadline(()=>cdp.send('Runtime.callFunctionOn',{
    objectId:remote.object.objectId,
    functionDeclaration:'function(){this.click();return {clicked:true};}',
    returnByValue:true,awaitPromise:false
   }),2000,'L19_NATIVE_SWITCH_UNCONFIRMED');
   if(called.exceptionDetails)operationResult={operation:'switch-record',exception:safe(called.exceptionDetails.exception?.description||called.exceptionDetails.text)};
   else if(called.result?.value?.clicked===true)operationResult={operation:'switch-record',clicked:true};
   else throw new Error('L19_NATIVE_SWITCH_UNCONFIRMED');
  }
  observationComplete=true;
  return {...witness,observationWindowMs:8000,rendererPaused:true,operationResult};
 }catch(error){primaryError=error;throw error;}finally{
  cdp.removeListener('Debugger.paused',onPaused);
  try{await l19Deadline(()=>cdp.send('Debugger.resume'),1000,'L19_NATIVE_RESUME_FAILED');}
  catch(error){
   if(!primaryError&&!observationComplete)throw error;
   if(primaryError)primaryError.l19Cleanup=l19Exception(error,'Debugger.resume');
   // A bounded native DOM/operation answer already exists. Keep the failed
   // cleanup step in RPC diagnostics; session exit terminates this renderer.
  }
 }
}
async function l19FrameSession(page,frame){
 // Chromium has separate targets for OOPIFs, but same-process children share
 // their parent's target. Fall back only on Playwright's exact shared-target
 // error, never on an inspector timeout or arbitrary attachment failure.
 let target=frame;
 for(let depth=0;depth<8;depth++){
  try{return await l19Deadline(()=>context.newCDPSession(target),2000,'CDP_CREATE_SESSION');}
  catch(error){
   if(error.message!=="This frame does not have a separate CDP session, it is a part of the parent frame's session"||target===page)throw error;
   target=target.parentFrame()||page;
   rpcState.getStore().l19Trace.push({operation:'resolve shared frame inspector target',outcome:'returned'});
  }
 }
 throw new Error('L19_NATIVE_FRAME_TARGET_UNAVAILABLE');
}
async function l19BoundedForm(req){
 const state=rpcState.getStore(),{page}=state;
 state.l19Trace=[];state.l19LastSnapshot=null;
 if(!['snapshot','switch-record'].includes(req.operation))throw new Error('L19_BOUNDED_OPERATION_INVALID');
 const frame=page.frames()[req.frame];
 if(new URL(frame.url()).pathname.toLowerCase()!==req.path.toLowerCase())throw new Error('L19_NATIVE_PATH_MISMATCH');
 const cdp=await l19FrameSession(page,frame);
 let primaryError,owned,nativeAnswerComplete=false;
 try{
  await l19Deadline(()=>cdp.send('Debugger.enable'),2000,'L19_NATIVE_INSPECTOR_UNAVAILABLE');
  const initial=await l19Deadline(()=>cdp.send('DOM.getDocument',{depth:-1,pierce:true}),2000,'L19_NATIVE_DOM_UNAVAILABLE');
  try{owned=l19OwnedHost(initial.root);}
  catch(error){error.l19Operation='verify native fixture identity before mount';throw error;}
  state.l19Trace.push({operation:'verify native fixture identity before mount',outcome:'returned',scope:'owned frame',hosts:1});
  const mounted=await l19Deadline(()=>frame.locator('[data-l19-host]:visible').evaluate((root,state)=>{
   const input=root.querySelector('[data-case-state]'),start=root.querySelector('[data-start]');
   if(!input||!start)throw new Error('L19 batch shell unavailable');
   input.value=JSON.stringify(state);start.click();
   return {id:state.c__case,started:true};
  },req.state),2000,'L19_NATIVE_MOUNT_UNCONFIRMED');
  if(mounted?.id!==req.state.c__case||mounted.started!==true)throw new Error('L19_NATIVE_MOUNT_UNCONFIRMED');
  const {operationResult,...before}=await l19NativeWindow(cdp,req.state.c__case,owned,req.operation==='switch-record');
  let after=null,action=operationResult||null;
  if(req.operation==='switch-record'){
   // Use the existing controller's real switch operation, never substitute a
   // different record into a screenshot and call that a record switch.
   after=await l19NativeWindow(cdp,req.state.c__case,owned);
  }
  if(new URL(frame.url()).pathname.toLowerCase()!==req.path.toLowerCase())throw new Error('L19_NATIVE_PATH_MISMATCH');
  nativeAnswerComplete=true;
  return {value:{id:req.state.c__case,stage:'bounded-native-observation',
   nativeEvidence:'Chromium DOM.getDocument',operation:req.operation,before,action,after}};
 }catch(error){
  primaryError=error;
  try{
   const doc=await l19Deadline(()=>cdp.send('DOM.getDocument',{depth:-1,pierce:true}),2000,'L19_NATIVE_DOM_UNAVAILABLE');
   try{state.l19FinalSnapshot={status:'captured',value:l19Witness(doc.root,req.state.c__case,owned)};}
   catch(witnessError){state.l19FinalSnapshot={status:'owned-state-unavailable',rootTag:doc.root.nodeName,
    failure:l19Exception(witnessError,'project final owned DOM')};}
  }catch(snapshotError){state.l19FinalSnapshot={status:'unavailable',failure:l19Exception(snapshotError,'final DOM.getDocument')};}
  throw error;
 }finally{
  try{await l19Deadline(()=>cdp.detach(),1000,'L19_NATIVE_INSPECTOR_DETACH_FAILED');}
  catch(error){
   if(!primaryError&&!nativeAnswerComplete)throw error;
   if(primaryError)primaryError.l19Cleanup=l19Exception(error,'CDPSession.detach');
   // Preserve proven native evidence; detach failures are timed, classified
   // diagnostic steps, not native answer text. The per-row child is stopped.
  }
 }
}

export async function observeInspector(page, state, operation) {
  return rpcState.run({ page }, async () => {
    const result = await l19BoundedForm({
      frame: 0, path: new URL(page.url()).pathname, state, operation,
    });
    return result.value;
  });
}
