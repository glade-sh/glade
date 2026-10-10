import { LightningElement, api } from 'lwc';
import { RefreshEvent, registerRefreshHandler, unregisterRefreshHandler, registerRefreshContainer, unregisterRefreshContainer } from 'lightning/refresh';
export default class FamilyRefreshView extends LightningElement {
 key=''; mode='true'; handle; handles=[]; containers=[]; hook; config;
 @api configure(key,mode,hook){this.key=key;this.mode=mode;this.hook=hook;}
 @api descendants(){let all=[this];for(const child of this.template.querySelectorAll('[data-child]'))all=all.concat(child.descendants());return all;}
 trace(phase,extra={}){this.dispatchEvent(new CustomEvent('trace',{detail:{node:this.key,phase,...extra},bubbles:true,composed:true}));}
 callback(){
  this.trace('start');if(this.hook)this.hook(this.key);
  const finish=value=>{this.trace('end',{result:value===undefined?'<undefined>':value});return value;};
  if(this.mode==='throw'){this.trace('throw');throw new Error('OWNED_HANDLER');}
  if(this.mode==='reject'){this.trace('reject');return Promise.reject(new Error('OWNED_HANDLER'));}
  if(this.mode==='never')return new Promise(()=>{});
  if(this.mode==='plain_true')return finish(true);
  if(this.mode==='delay_true'||this.mode==='delay_false')return new Promise(resolve=>setTimeout(()=>resolve(finish(this.mode==='delay_true')),20));
  let value=true;
  if(this.mode==='false')value=false;
  if(this.mode==='resolved_null')value=null;
  if(this.mode==='resolved_undefined')value=undefined;
  if(this.mode==='resolved_string')value='true';
  if(this.mode==='resolved_zero')value=0;
  if(this.mode==='resolved_one')value=1;
  return Promise.resolve(value).then(finish);
 }
 @api register(format){const cb=this.callback.bind(this);this.handle=registerRefreshHandler(this,format==='locker'?{handler:cb}:cb);this.handles.push(this.handle);return this.handle;}
 @api unregister(value,useValue){return unregisterRefreshHandler(useValue?value:this.handle);}
 @api container(callback,format){const h=registerRefreshContainer(this,format==='locker'?{handler:callback}:callback);this.containers.push(h);return h;}
 @api uncontainer(handle){return unregisterRefreshContainer(handle);}
 @api emit(kind,options,action){
  let event=kind==='custom'?new CustomEvent('refresh',{bubbles:true,composed:true,cancelable:true}):kind==='generic'?new Event('refresh',{bubbles:true,composed:true,cancelable:true}):new RefreshEvent(options);
  if(action==='prevent')event.preventDefault();
  if(action==='stop')event.stopPropagation();
  if(action==='immediate')event.stopImmediatePropagation();
  const listen=e=>{if(action==='listener_cancel')e.preventDefault();if(action==='listener_stop')e.stopPropagation();if(action==='listener_immediate')e.stopImmediatePropagation();};
  this.addEventListener(event.type,listen,{once:true});
  try {const returned=this.dispatchEvent(event);return {returned,type:event.type,bubbles:event.bubbles,composed:event.composed,cancelable:event.cancelable,defaultPrevented:event.defaultPrevented};}
  finally {this.removeEventListener(event.type,listen);}
 }
 @api cleanup(){const errors=[];for(const h of this.handles){try{unregisterRefreshHandler(h);}catch(e){errors.push({name:e.name,message:String(e.message)});}}this.handles=[];this.handle=undefined;
  for(const h of this.containers){try{unregisterRefreshContainer(h);}catch(e){errors.push({name:e.name,message:String(e.message)});}}this.containers=[];return errors;}

 @api registrationControl(registry,mode,firstCallback,secondCallback){
  const register=registry==='handler'?registerRefreshHandler:registerRefreshContainer;
  const unregister=registry==='handler'?unregisterRefreshHandler:unregisterRefreshContainer;
  const owned=registry==='handler'?this.handles:this.containers;
  const describe=v=>v===undefined?{type:'undefined',value:'<undefined>'}:typeof v==='number'?{type:'number',finite:Number.isFinite(v),integer:Number.isInteger(v)}:{type:typeof v,value:v};
  const error=e=>{const message=e&&e.message!==undefined?e.message:String(e);return {name:e&&e.name!==undefined?e.name:typeof e,message:typeof message==='string'?message.replace(/https?:\/\/[^\s]+/g,'<URL>'):message};};
  const provider=(label,callback)=>registry==='handler'?()=>{this.trace('control-provider',{provider:label});return this.callback();}:p=>{this.trace('control-provider',{provider:label});return callback(p);};
  const firstProvider=provider('first',firstCallback),secondProvider=mode==='duplicate_new'?provider('second',secondCallback):firstProvider;
  const attempts=[];let first,firstSucceeded=false;
  const attempt=(phase,callback)=>{try{const handle=register(this,callback);owned.push(handle);attempts.push({phase,returned:describe(handle),sameAsFirst:firstSucceeded?handle===first:null});return {success:true,handle};}
   catch(e){attempts.push({phase,error:error(e)});return {success:false};}};
  const initial=attempt('first-direct-function',firstProvider);first=initial.handle;firstSucceeded=initial.success;
  if(mode==='reregister_same'){try{attempts.push({phase:'unregister-first',returned:describe(unregister(first))});}catch(e){attempts.push({phase:'unregister-first',error:error(e)});}}
  attempt('second-direct-function',secondProvider);
  return {registry,mode,context:'LightningElement',sameProvider:secondProvider===firstProvider,attempts};
 }
 disconnectedCallback(){this.cleanup();}
}
