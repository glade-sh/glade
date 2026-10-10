import { LightningElement } from 'lwc';
export default class FamilyApi extends LightningElement {
 result=''; _records=[]; _running=false;
 encode(v) {
  if(v===undefined)return {type:'undefined'};
  if(v===null)return null;
  if(typeof v==='number' && (!Number.isFinite(v)||Object.is(v,-0)))return {type:'number',value:Object.is(v,-0)?'-0':String(v)};
  if(Array.isArray(v))return v.map(x=>this.encode(x));
  if(typeof v==='object'){const r={};for(const k of Object.keys(v).sort())r[k]=this.encode(v[k]);return r;}
  return v;
 }
 errorCallback(e){this._records.push({error:{name:e.name,message:e.message}});}
 async run(){
  if(this._running)return;this._running=true;this._records=[];
  const record=(label,v)=>this._records.push({label,value:this.encode(v)});
  const recordError=e=>this._records.push({error:{name:e.name,message:e.message}});
  const child=this.template.querySelector('__CHILD__');
  const wrapper=this.template.querySelector('[data-node="wrapper"]');
  const label=n=>n===child?'child-host':n===wrapper?'wrapper':n&&n.dataset&&n.dataset.node==='inside'?'inside':'outside';
  const installed=[];
  const installListeners=()=>{
   const listen=(node,where,capture)=>{
    const fn=e=>{try {__LISTENER_ACTION__} catch(error){recordError(error);}
     record(where,{type:e.type,target:label(e.target),currentTarget:label(e.currentTarget),phase:e.eventPhase,
      bubbles:e.bubbles,composed:e.composed,cancelable:e.cancelable,defaultPrevented:e.defaultPrevented,
      detail:e.detail,path:e.composedPath().map(label).filter(x=>x!=='outside')});};
    node.addEventListener(__EVENT__,fn,capture);installed.push([node,fn,capture]);
   };
   listen(wrapper,'wrapper-capture',true);listen(child,'child-listener',false);listen(child,'child-second',false);listen(wrapper,'wrapper-bubble',false);
  };
  try {if(!child)throw new Error('owned child missing');__ACTION__}catch(e){recordError(e);}
  for(const [node,fn,capture] of installed)node.removeEventListener(__EVENT__,fn,capture);
  await new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve)));
  this.result=JSON.stringify({id:__ID__,records:this._records,childText:child?child.textContent:null});
  this._running=false;
 }
}
