import { LightningElement, wire } from 'lwc';
import { CurrentPageReference } from 'lightning/navigation';
import { getRecord, getFieldValue } from 'lightning/uiRecordApi';
const SPECS=[{"id": "r_form_view_new_auto", "group": "form modes", "kind": "runtime", "scenario": "normal", "expect": "?", "basis": "org", "component": "form", "record": "new", "props": {"mode": "view", "density": "auto"}, "object": "Account", "operation": "snapshot", "fields": ["Name"], "observation": "Native public properties/events, semantic DOM, owned-record readback; bounded observations retain their window, never become exceptions"}, {"id": "r_form_view_new_comfy", "group": "form modes", "kind": "runtime", "scenario": "normal", "expect": "?", "basis": "org", "component": "form", "record": "new", "props": {"mode": "view", "density": "comfy"}, "object": "Account", "operation": "snapshot", "fields": ["Name"], "observation": "Native public properties/events, semantic DOM, owned-record readback; bounded observations retain their window, never become exceptions"}, {"id": "r_form_view_new_compact", "group": "form modes", "kind": "runtime", "scenario": "normal", "expect": "?", "basis": "org", "component": "form", "record": "new", "props": {"mode": "view", "density": "compact"}, "object": "Account", "operation": "snapshot", "fields": ["Name"], "observation": "Native public properties/events, semantic DOM, owned-record readback; bounded observations retain their window, never become exceptions"}, {"id": "r_form_edit_new_auto", "group": "form modes", "kind": "runtime", "scenario": "normal", "expect": "?", "basis": "org", "component": "form", "record": "new", "props": {"mode": "edit", "density": "auto"}, "object": "Account", "operation": "snapshot", "fields": ["Name"], "observation": "Native public properties/events, semantic DOM, owned-record readback; bounded observations retain their window, never become exceptions"}, {"id": "r_form_edit_new_comfy", "group": "form modes", "kind": "runtime", "scenario": "normal", "expect": "?", "basis": "org", "component": "form", "record": "new", "props": {"mode": "edit", "density": "comfy"}, "object": "Account", "operation": "snapshot", "fields": ["Name"], "observation": "Native public properties/events, semantic DOM, owned-record readback; bounded observations retain their window, never become exceptions"}, {"id": "r_form_edit_new_compact", "group": "form modes", "kind": "runtime", "scenario": "normal", "expect": "?", "basis": "org", "component": "form", "record": "new", "props": {"mode": "edit", "density": "compact"}, "object": "Account", "operation": "snapshot", "fields": ["Name"], "observation": "Native public properties/events, semantic DOM, owned-record readback; bounded observations retain their window, never become exceptions"}, {"id": "r_form_readonly_new_auto", "group": "form modes", "kind": "runtime", "scenario": "normal", "expect": "?", "basis": "org", "component": "form", "record": "new", "props": {"mode": "readonly", "density": "auto"}, "object": "Account", "operation": "snapshot", "fields": ["Name"], "observation": "Native public properties/events, semantic DOM, owned-record readback; bounded observations retain their window, never become exceptions"}, {"id": "r_form_readonly_new_comfy", "group": "form modes", "kind": "runtime", "scenario": "normal", "expect": "?", "basis": "org", "component": "form", "record": "new", "props": {"mode": "readonly", "density": "comfy"}, "object": "Account", "operation": "snapshot", "fields": ["Name"], "observation": "Native public properties/events, semantic DOM, owned-record readback; bounded observations retain their window, never become exceptions"}, {"id": "r_form_readonly_new_compact", "group": "form modes", "kind": "runtime", "scenario": "normal", "expect": "?", "basis": "org", "component": "form", "record": "new", "props": {"mode": "readonly", "density": "compact"}, "object": "Account", "operation": "snapshot", "fields": ["Name"], "observation": "Native public properties/events, semantic DOM, owned-record readback; bounded observations retain their window, never become exceptions"}, {"id": "r_submit_create_name", "group": "submit/error flows", "kind": "runtime", "scenario": "normal", "expect": "?", "basis": "org", "component": "edit", "record": "new", "operation": "submit", "payload": {"Name": "$OWNED_SUBMITTED"}, "object": "Account", "fields": ["Name"], "props": {}, "observation": "Native public properties/events, semantic DOM, owned-record readback; bounded observations retain their window, never become exceptions"}, {"id": "r_submit_create_null", "group": "submit/error flows", "kind": "runtime", "scenario": "boundary", "expect": "?", "basis": "org", "component": "edit", "record": "new", "operation": "submit", "payload": {"Name": null}, "object": "Account", "fields": ["Name"], "props": {}, "observation": "Native public properties/events, semantic DOM, owned-record readback; bounded observations retain their window, never become exceptions"}, {"id": "r_submit_create_missing", "group": "submit/error flows", "kind": "runtime", "scenario": "boundary", "expect": "?", "basis": "org", "component": "edit", "record": "new", "operation": "submit", "payload": {}, "object": "Account", "fields": ["Name"], "props": {}, "observation": "Native public properties/events, semantic DOM, owned-record readback; bounded observations retain their window, never become exceptions"}, {"id": "r_submit_create_empty", "group": "submit/error flows", "kind": "runtime", "scenario": "boundary", "expect": "?", "basis": "org", "component": "edit", "record": "new", "operation": "submit", "payload": {"Name": ""}, "object": "Account", "fields": ["Name"], "props": {}, "observation": "Native public properties/events, semantic DOM, owned-record readback; bounded observations retain their window, never become exceptions"}, {"id": "r_submit_name_max", "group": "submit/error flows", "kind": "runtime", "scenario": "normal", "expect": "?", "basis": "org", "component": "edit", "record": "new", "operation": "submit", "payload": {"Name": "NNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNN"}, "object": "Account", "fields": ["Name"], "props": {}, "observation": "Native public properties/events, semantic DOM, owned-record readback; bounded observations retain their window, never become exceptions"}, {"id": "r_submit_name_overlong", "group": "submit/error flows", "kind": "runtime", "scenario": "boundary", "expect": "?", "basis": "org", "component": "edit", "record": "new", "operation": "submit", "payload": {"Name": "NNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNN"}, "object": "Account", "fields": ["Name"], "props": {}, "observation": "Native public properties/events, semantic DOM, owned-record readback; bounded observations retain their window, never become exceptions"}, {"id": "r_submit_name_unicode", "group": "submit/error flows", "kind": "runtime", "scenario": "normal", "expect": "?", "basis": "org", "component": "edit", "record": "new", "operation": "submit", "payload": {"Name": "\u00e9\u6c34\ud83d\ude00"}, "object": "Account", "fields": ["Name"], "props": {}, "observation": "Native public properties/events, semantic DOM, owned-record readback; bounded observations retain their window, never become exceptions"}, {"id": "r_submit_name_html", "group": "submit/error flows", "kind": "runtime", "scenario": "normal", "expect": "?", "basis": "org", "component": "edit", "record": "new", "operation": "submit", "payload": {"Name": "<b>& owned</b>"}, "object": "Account", "fields": ["Name"], "props": {}, "observation": "Native public properties/events, semantic DOM, owned-record readback; bounded observations retain their window, never become exceptions"}, {"id": "r_submit_readonly_created", "group": "submit/error flows", "kind": "runtime", "scenario": "boundary", "expect": "?", "basis": "org", "component": "edit", "record": "existing", "operation": "submit", "payload": {"CreatedDate": "2001-01-01T00:00:00.000Z"}, "object": "Account", "fields": ["Name"], "props": {}, "observation": "Native public properties/events, semantic DOM, owned-record readback; bounded observations retain their window, never become exceptions"}, {"id": "r_submit_contact_lastname_number", "group": "submit/error flows", "kind": "runtime", "scenario": "normal", "expect": "?", "basis": "org", "component": "edit", "object": "Contact", "fields": ["LastName"], "record": "new", "operation": "submit", "payload": {"LastName": 42}, "props": {}, "observation": "Native public properties/events, semantic DOM, owned-record readback; bounded observations retain their window, never become exceptions"}];
export default class FamilyL19ControlLoads extends LightningElement {
 spec; mounted=false; result=''; events=[]; account; second; contact; ownedName;
 readbackId; persisted; readbackError; marker;
 objectName; recordId; fields; density='auto'; mode; columns; layoutType;
 fieldValue; required=false; disabled=false; readOnly=false; variant='standard';
 label='Owned picker'; pickerValue; placeholder; displayInfo; matchingInfo; filter;
 stage='initializing'; apiReturns=[]; readyTimer; eventTrace=[]; observationOrigin;
 async startNext() {
  const state=JSON.parse(this.template.querySelector('[data-case-state]').value);
  if(!SPECS.some(c=>c.id===state.c__case))return;
  clearTimeout(this.readyTimer);
  this.mounted=false;
  // Let LWC disconnect the previous native component before the next mount.
  await new Promise(resolve=>setTimeout(resolve,0));
  Object.assign(this,{spec:undefined,result:'',events:[],eventTrace:[],apiReturns:[],stage:'initializing',
   account:undefined,second:undefined,contact:undefined,ownedName:undefined,marker:undefined,
   readbackId:undefined,persisted:undefined,readbackError:undefined,objectName:undefined,
   recordId:undefined,fields:undefined,density:'auto',mode:undefined,columns:undefined,
   layoutType:undefined,fieldValue:undefined,required:false,disabled:false,readOnly:false,
   variant:'standard',label:'Owned picker',pickerValue:undefined,placeholder:undefined,
   displayInfo:undefined,matchingInfo:undefined,filter:undefined});
  this.page({state});
 }
 @wire(CurrentPageReference) page(reference) {
  if(this.spec || !reference || !reference.state)return;
  const s=reference.state;this.observationOrigin=Date.now();
  this.spec=SPECS.find(c=>c.id===s.c__case);
  if(!this.spec)return;
  this.account=s.c__account;this.second=s.c__second;this.contact=s.c__contact;this.ownedName=s.c__name;this.marker=s.c__marker;
  this.objectName=this.spec.object;
  this.recordId=this.spec.record==='new' ? undefined : this.spec.record==='account-mismatch' ? this.account : this.spec.object==='Contact' ? this.contact : this.account;
  this.fields=this.spec.fields;this.readbackId=this.recordId;
  const p=this.spec.props;
  for(const key of ['density','mode','columns','layoutType','fieldValue','required','disabled','readOnly','variant','label','placeholder','displayInfo','matchingInfo','filter']) {
   if(Object.prototype.hasOwnProperty.call(p,key))this[key]=this.resolve(p[key]);
  }
  if(Object.prototype.hasOwnProperty.call(p,'value'))this.pickerValue=this.resolve(p.value);
  this.mounted=true;
  // This timer records a page observation even if no load/ready/error arrives.
  this.readyTimer=setTimeout(()=>{this.stage='ready';this.publish();},5000);
 }
 resolve(value) {
  if(value==='$ACCOUNT')return this.account;
  if(value==='$SECOND')return this.second;
  if(value==='$OWNED_SUBMITTED')return this.ownedName+' SUBMITTED';
  if(Array.isArray(value))return value.map(v=>this.resolve(v));
  if(value && typeof value==='object')return Object.fromEntries(Object.entries(value).map(([k,v])=>[k,this.resolve(v)]));
  return value;
 }
 get component(){return this.spec && this.spec.component;}
 get isForm(){return this.spec && this.spec.component==='form';}
 get isEdit(){return this.spec && ['edit','input'].includes(this.spec.component);}
 get isView(){return this.spec && ['view','output'].includes(this.spec.component);}
 get isPicker(){return this.spec && this.spec.component==='picker';}
 get fieldSpecs(){return (Array.isArray(this.fields) ? this.fields : []).map((name,i)=>({key:String(i),name}));}
 get captureFields(){return this.spec ? this.spec.object==='Contact' ? ['Contact.LastName','Contact.Email','Contact.Birthdate','Contact.AccountId'] : ['Account.Name','Account.Phone','Account.AnnualRevenue','Account.NumberOfEmployees','Account.Description'] : undefined;}
 @wire(getRecord,{recordId:'$readbackId',fields:'$captureFields'}) readback({data,error}) {
  if(data) {
   this.persisted=Object.fromEntries(this.captureFields.map(name=>[name.split('.')[1],getFieldValue(data,name)]));
  } else if(error)this.readbackError=this.errorShape(error);
 }
 errorShape(error){
  if(!error)return null;
  const b=error.body || error;
  return {status:error.status,errorCode:b.errorCode,message:b.message,output:b.output};
 }
 record(event){
  const detail=event.detail || {};
  const value={type:event.type,bubbles:event.bubbles,composed:event.composed,cancelable:event.cancelable};
  if(event.type==='submit')value.fields=detail.fields;
  if(event.type==='success'){value.recordId=detail.id;this.readbackId=detail.id;}
  if(event.type==='change')value.detail=detail;
  if(event.type==='error')value.error=this.errorShape(detail);
  this.events=[...this.events,value];this.eventTrace=[...this.eventTrace,{index:this.events.length-1,type:event.type,stage:this.stage,elapsedMs:Date.now()-this.observationOrigin,detailKeys:Object.keys(detail),recordIds:Object.keys(detail.records||{})}];
  if(event.type==='submit' && ['prevent-submit','override-submit'].includes(this.spec.operation)) {
   event.preventDefault();value.defaultPrevented=event.defaultPrevented;
   if(this.spec.operation==='override-submit')this.template.querySelector('lightning-record-edit-form').submit(this.submitPayload());
  }
  this.publish();
 }
 submitPayload(){const fields=this.resolve(this.spec.payload);return this.spec.record==='new'?{...fields,[this.spec.object==='Contact'?'Department':'AccountNumber']:this.marker}:fields;}
 errorCallback(error){this.events=[...this.events,{type:'render-error',error:{name:error.name,message:error.message}}];this.publish();}
 publicState(){
  const picker=this.template.querySelector('lightning-record-picker');
  const fields=Array.from(this.template.querySelectorAll('lightning-input-field')).map(f=>({fieldName:f.fieldName,value:f.value,required:f.required,disabled:f.disabled,readOnly:f.readOnly,variant:f.variant}));
  return {fields,picker:picker ? {value:picker.value,required:picker.required,disabled:picker.disabled}:null};
 }
 publish(){
  this.result=JSON.stringify({id:this.spec.id,sourceApi:'59.0',eventTrace:this.eventTrace,stage:this.stage,events:this.events,apiReturns:this.apiReturns,publicState:this.publicState(),persisted:this.persisted,readbackError:this.readbackError});
 }
 execute(){
  const op=this.spec.operation;
  this.stage='acting';
  try {
   const field=this.template.querySelector('lightning-input-field');
   const picker=this.template.querySelector('lightning-record-picker');
   const edit=this.template.querySelector('lightning-record-edit-form');
   if(op==='submit' || op==='submit-deleted')edit.submit(this.submitPayload());
   else if(op==='reset') {for(const f of this.template.querySelectorAll('lightning-input-field'))f.reset();this.apiReturns.push({method:'reset',called:true});}
   else if(op==='report-validity')this.apiReturns.push({method:'reportValidity',result:(picker || field).reportValidity()});
   else if(op==='focus') {picker.focus();this.apiReturns.push({method:'focus',called:true});}
   else if(op==='custom-validity' || op==='custom-validity-clear') {
    picker.setCustomValidity(this.spec.props.customValidity);this.apiReturns.push({method:'reportValidity',phase:'set',result:picker.reportValidity()});
    if(op==='custom-validity-clear'){picker.setCustomValidity('');this.apiReturns.push({method:'reportValidity',phase:'clear',result:picker.reportValidity()});}
   } else if(op==='switch-record'){this.recordId=this.second;this.readbackId=this.second;}
  } catch(error){this.apiReturns.push({exception:{name:error.name,message:error.message}});}
  // Finish only on an explicit coordinator click after its DOM interactions.
  this.publish();
 }
 finish(){this.stage='done';this.publish();}
}
