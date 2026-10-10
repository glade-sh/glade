import {LightningElement} from 'lwc';import {trace} from 'c/__TRACE__';
export default class FamilyLifecycle extends LightningElement{
 show=false; result=''; _child=null; _index=0; _errors=[];
 mount(){this.show=true;}
 unmount(){this._child=this.template.querySelector('__CHILD__')||this._child;this.show=false;}
 advance(){
  const child=this.template.querySelector('__CHILD__');
  if(!child){this._errors.push({kind:'control',childPresent:false});return;}
  try{const result=child.advance(this._index++);if(result&&typeof result.catch==='function')result.catch(e=>this.record(e,'control-promise'));}
  catch(error){this.record(error,'control');}
 }
 fire(){const child=this.template.querySelector('__CHILD__');if(child)child.fire();}
 record(error,kind){this._errors.push({kind,name:error.name,message:error.message});}
 errorCallback(error,stack){this.record(error,'boundary:errorCallback');trace.push({callback:'boundary:errorCallback',name:error.name,message:error.message,stackPresent:typeof stack==='string'&&stack.length>0});}
 snapshot(){
  const child=this.template.querySelector('__CHILD__');this._child=child||this._child;
  let dom=[];try{if(this._child)dom=this._child.snapshot();}catch(error){this.record(error,'snapshot');}
  this.result=JSON.stringify({dom,trace:trace.slice(),errors:this._errors.slice(),childPresent:!!child,connected:!!(this._child&&this._child.isConnected)});
 }
}
