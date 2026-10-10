import { LightningElement, wire } from 'lwc';
import { CurrentPageReference } from 'lightning/navigation';
import { RefreshEvent, registerRefreshHandler, unregisterRefreshHandler, registerRefreshContainer, unregisterRefreshContainer, RefreshComplete, RefreshCompleteWithError, RefreshError } from 'lightning/refresh';
const SPECS=__SPECS__;
export default class FamilyRefreshView extends LightningElement {
 ready=false;result='';spec;running=false;trace=[];observation;nodes={};handlerHandles=[];containerHandles=[];orphans=[];generation=0;
 @wire(CurrentPageReference) page(value){if(!value)return;const id=value.state&&value.state.c__case;const spec=SPECS.find(s=>s.id===id);if(!spec||this.spec&&this.spec.id===id)return;
  this.cleanup();this.generation++;this.spec=spec;this.ready=true;this.publish({id,stage:'ready'});}
 publish(value){this.result=JSON.stringify(value);}
 error(e){return {name:e&&e.name||typeof e,message:String(e&&e.message||e).replace(/https?:\/\/[^\s]+/g,'<URL>')};}
 value(v){if(v===undefined)return {type:'undefined',value:'<undefined>'};if(typeof v==='number')return {type:'number',finite:Number.isFinite(v),integer:Number.isInteger(v)};if(typeof v==='function')return {type:'function'};return {type:typeof v,value:v};}
 recordTrace(event){if(!this.running)return;this.trace.push({...event.detail});}
 registerNode(node,format='auto'){try{return {handle:node.register(format==='locker'?'locker':'lws'),signature:format==='locker'?'locker-object':'function'};}
  catch(e){if(format!=='auto')throw e;return {handle:node.register('locker'),signature:'locker-object',firstError:this.error(e)};}}
 registerContainer(node,callback,format='auto'){try{return {handle:node.container(callback,format==='locker'?'locker':'lws'),signature:format==='locker'?'locker-object':'function'};}
  catch(e){if(format!=='auto')throw e;return {handle:node.container(callback,'locker'),signature:'locker-object',firstError:this.error(e)};}}
 cleanup(){const errors=[];for(const node of Object.values(this.nodes||{})){try{errors.push(...node.cleanup());}catch(e){errors.push(this.error(e));}}
  for(const item of this.orphans||[]){try{(item.registry==='handler'?unregisterRefreshHandler:unregisterRefreshContainer)(item.handle);}catch(e){errors.push(this.error(e));}}this.orphans=[];return errors;}
 async execute(){if(this.running)return;this.running=true;this.trace=[];this.handlerHandles=[];this.containerHandles=[];const spec=this.spec;const epoch=this.generation;
  const obs={constants:{complete:{type:typeof RefreshComplete,value:RefreshComplete},withError:{type:typeof RefreshCompleteWithError,value:RefreshCompleteWithError},error:{type:typeof RefreshError,value:RefreshError},distinct:RefreshComplete!==RefreshError&&RefreshComplete!==RefreshCompleteWithError&&RefreshError!==RefreshCompleteWithError},registrations:[],completions:[],operations:[],events:[]};this.observation=obs;
  const root=this.template.querySelector('[data-root]'),outside=this.template.querySelector('[data-outside]');
  const names=['root','a','a1','a2','b','b1','b2'];const nodes=root.descendants();this.nodes={};
  if(nodes.length!==names.length){this.publish({id:spec.id,stage:'done',captureError:'FIXED_TREE_INCOMPLETE'});this.running=false;return;}
  nodes.forEach((node,i)=>{this.nodes[names[i]]=node;node.configure(names[i],(spec.modes||{})[names[i]]||'true',key=>this.midRefresh(key,spec));});
  this.nodes.outside=outside;outside.configure('outside','true',null);
  const callback=key=>p=>{obs.operations.push({phase:'container-start',node:key,promise:this.value(p instanceof Promise),thenType:typeof(p&&p.then)});
   if(spec.action==='callback_throw')throw new Error('OWNED_CONTAINER');
   if(spec.action==='callback_ignore')return undefined;
   Promise.resolve(p).then(status=>{if(this.generation!==epoch)return;obs.completions.push({node:key,type:typeof status,value:status,complete:status===RefreshComplete,withError:status===RefreshCompleteWithError,error:status===RefreshError});},e=>{if(this.generation===epoch)obs.completions.push({node:key,rejection:this.error(e)});});
   if(spec.action==='callback_async')return Promise.resolve(p);return undefined;};
  try{
   if(spec.operation==='registration_control'){
    await this.registrationControl(spec,obs,root,callback);
   }else if(spec.operation==='constructor'){
    const event=new RefreshEvent(__EVENT_EXPRESSION__);obs.constructor={type:event.type,bubbles:event.bubbles,composed:event.composed,cancelable:event.cancelable,defaultPrevented:event.defaultPrevented,detail:this.value(event.detail),instance:event instanceof Event};
   }else if(spec.action==='constants'){
    obs.operations.push({phase:'constants-only'});
   }else{
    const format=spec.action==='locker'?'locker':'auto';
    if(!['none','no_container'].includes(spec.action)){
     const c=this.registerContainer(root,callback('root'),format);this.containerHandles.push(c.handle);obs.registrations.push({node:'root',registry:'container',value:this.value(c.handle),signature:c.signature,firstError:c.firstError});}
    let registered=spec.registered===undefined?['a','a1','a2','b','b1','b2']:[...spec.registered];if(spec.include_root)registered.unshift('root');if(spec.register_outside)registered.push('outside');
    if(spec.order==='reverse')registered.reverse();if(spec.order==='leaves_first')registered=['b2','b1','a2','a1','b','a'];
    for(const name of registered){const r=this.registerNode(this.nodes[name],format);this.handlerHandles.push(r.handle);obs.registrations.push({node:name,registry:'handler',value:this.value(r.handle),signature:r.signature,firstError:r.firstError});}
    obs.handlerDistinct=new Set(this.handlerHandles).size===this.handlerHandles.length;obs.crossRegistryEqual=this.handlerHandles.some(h=>this.containerHandles.includes(h));
    if(spec.action==='nested_container'){const c=this.registerContainer(this.nodes.a,callback('a'),format);this.containerHandles.push(c.handle);obs.registrations.push({node:'a',registry:'container',value:this.value(c.handle),signature:c.signature});}
    const registry=spec.registry||'handler',node=registry==='handler'?this.nodes.a:root;
    const reg=()=>registry==='handler'?this.registerNode(node,format):this.registerContainer(node,callback('repeat-root'),format);
    const unreg=h=>registry==='handler'?node.unregister(h,true):node.uncontainer(h);
    const owned=registry==='handler'?this.handlerHandles[0]:this.containerHandles[0];
    if(spec.action==='invalid_unregister'){const result=unreg(__VALUE_EXPRESSION__);obs.operations.push({phase:'invalid-unregister',returned:this.value(result)});}
    if(spec.action==='invalid_register'){const f=registry==='handler'?registerRefreshHandler:registerRefreshContainer;const h=f(__CONTEXT_EXPRESSION__,__PROVIDER_EXPRESSION__);this.orphans.push({registry,handle:h});obs.operations.push({phase:'invalid-register',returned:this.value(h)});}
    if(['distinct','duplicate','reregister'].includes(spec.action)){if(spec.action==='reregister')unreg(owned);const one=reg(),two=spec.action==='distinct'?(registry==='handler'?this.registerNode(this.nodes.b,format):this.registerContainer(this.nodes.b,callback('distinct-b'),format)):null;obs.operations.push({phase:spec.action,returned:this.value(one.handle),differsFromOriginal:one.handle!==owned,second:two?this.value(two.handle):null,distinct:two?one.handle!==two.handle:null});}
    if(['unregister','double_unregister','unregister_before','unregister_container'].includes(spec.action)){
     const h=spec.action==='unregister_container'?this.containerHandles[0]:owned;obs.operations.push({phase:'unregister',returned:this.value(spec.action==='unregister_container'?root.uncontainer(h):unreg(h))});
     if(spec.action==='double_unregister')obs.operations.push({phase:'unregister-again',returned:this.value(unreg(h))});}
    if(spec.action==='cross_unregister')obs.operations.push({phase:'cross-unregister',returned:this.value((registry==='handler'?unregisterRefreshContainer:unregisterRefreshHandler)(owned))});
    if(spec.action==='unregister_parent')this.nodes.a.unregister();if(spec.action==='unregister_leaf')this.nodes.a1.unregister();
    const target=this.nodes[spec.target||'a1'];obs.events.push(target.emit(spec.event||'refresh',undefined,spec.action));
    // Native absence is an answer with a named bounded observation, never an inferred status/error.
    await new Promise(resolve=>{const started=Date.now();const poll=()=>{if(obs.completions.length||Date.now()-started>=4000)resolve();else setTimeout(poll,10);};poll();});
    await new Promise(resolve=>setTimeout(resolve,40));
    obs.boundary=obs.completions.length?'completion-observed':'NO_OWNED_COMPLETION_WITHIN_4000MS';
    if(spec.action==='unregister_after')obs.operations.push({phase:'unregister-after',returned:this.value(unreg(owned))});
    if(spec.action==='repeat'){obs.events.push(target.emit('refresh',undefined,null));await new Promise(resolve=>setTimeout(resolve,100));}
   }
  }catch(e){obs.exception=this.error(e);}
  obs.trace=[...this.trace];obs.cleanupErrors=this.cleanup();this.running=false;
  this.publish({id:spec.id,stage:'done',observation:obs});
 }

 async registrationControl(spec,obs,root,callback){
  const registry=spec.registry,node=registry==='handler'?this.nodes.a:root;
  if(registry==='handler'){
   const c=this.registerContainer(root,callback('root'),'lws');this.containerHandles.push(c.handle);obs.registrations.push({node:'root',registry:'container',value:this.value(c.handle),signature:c.signature});
  }else{
   const h=this.registerNode(this.nodes.a,'lws');this.handlerHandles.push(h.handle);obs.registrations.push({node:'a',registry:'handler',value:this.value(h.handle),signature:h.signature});
  }
  obs.operations.push({phase:'registration-control',node:registry==='handler'?'a':'root',...node.registrationControl(registry,spec.control_mode,callback('first-root'),callback('second-root'))});
  obs.events.push(this.nodes.a1.emit('refresh',undefined,null));
  await new Promise(resolve=>{const started=Date.now();const poll=()=>{if(obs.completions.length||Date.now()-started>=4000)resolve();else setTimeout(poll,10);};poll();});
  await new Promise(resolve=>setTimeout(resolve,40));
  obs.boundary=obs.completions.length?'completion-observed':'NO_OWNED_COMPLETION_WITHIN_4000MS';
 }
 midRefresh(key,spec){const action=spec.action;if(key!=='a'||!['mid_unregister','self_unregister'].includes(action))return;
  try{const result=spec.registry==='container'?this.nodes.root.uncontainer(this.containerHandles[0]):action==='self_unregister'?this.nodes.a.unregister():this.nodes.a1.unregister();this.observation.operations.push({phase:action,node:key,returned:this.value(result)});}
  catch(e){this.observation.operations.push({phase:action,error:this.error(e)});}}
 disconnectedCallback(){this.generation++;this.cleanup();}
}
