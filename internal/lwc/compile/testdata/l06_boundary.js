// Owned boundary fixture exported from the native DOM-composition capture.
import { LightningElement } from 'lwc';
export default class FamilyDom extends LightningElement {
 result=''; _requested=0; _steps=[]; _errors=[];
 _publish(){ this.result=JSON.stringify({id:__CASE__,requested:this._requested,steps:this._steps,errors:this._errors}); }
 handleObserve(event){ this._steps.push(event.detail); this._publish(); }
 errorCallback(error){ this._errors.push({step:this._requested,name:error.name,message:error.message}); this._publish(); }
 advance(){
  this._requested+=1;
  const child=this.template.querySelector('__CHILD__');
  if(!child){ this._errors.push({step:this._requested,childPresent:false}); this._publish(); return; }
  try{ child.advance(); }catch(e){ this._errors.push({step:this._requested,name:e.name,message:e.message}); this._publish(); }
 }
}
