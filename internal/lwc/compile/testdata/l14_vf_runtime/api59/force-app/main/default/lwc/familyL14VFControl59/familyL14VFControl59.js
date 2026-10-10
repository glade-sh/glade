import { LightningElement, api } from 'lwc';
import { NavigationMixin } from 'lightning/navigation';
const SPECS = [{"id": "page_object_Account_new", "host": "Visualforce Lightning Out", "operation": "generate", "expect": "?", "page": {"type": "standard__objectPage", "attributes": {"objectApiName": "Account", "actionName": "new"}}}, {"id": "page_object_Account_home", "host": "Visualforce Lightning Out", "operation": "generate", "expect": "?", "page": {"type": "standard__objectPage", "attributes": {"objectApiName": "Account", "actionName": "home"}}}, {"id": "page_object_Account_list", "host": "Visualforce Lightning Out", "operation": "generate", "expect": "?", "page": {"type": "standard__objectPage", "attributes": {"objectApiName": "Account", "actionName": "list"}}}, {"id": "page_object_Account_view", "host": "Visualforce Lightning Out", "operation": "generate", "expect": "?", "page": {"type": "standard__objectPage", "attributes": {"objectApiName": "Account", "actionName": "view"}}}, {"id": "page_object_Account_", "host": "Visualforce Lightning Out", "operation": "generate", "expect": "?", "page": {"type": "standard__objectPage", "attributes": {"objectApiName": "Account", "actionName": ""}}}, {"id": "page_object_Account_None", "host": "Visualforce Lightning Out", "operation": "generate", "expect": "?", "page": {"type": "standard__objectPage", "attributes": {"objectApiName": "Account", "actionName": null}}}, {"id": "generate_null", "host": "Visualforce Lightning Out", "operation": "generate", "expect": "?", "page": null}, {"id": "generate_undefined", "host": "Visualforce Lightning Out", "operation": "generate", "expect": "?", "page_expr": "undefined"}, {"id": "generate_throwing_type", "host": "Visualforce Lightning Out", "operation": "generate", "expect": "?", "page_expr": "({get type(){throw new TypeError('L14 owned type getter');},attributes:{}})"}, {"id": "page_object_Account_missing_action", "page": {"type": "standard__objectPage", "attributes": {"objectApiName": "Account"}}, "host": "Visualforce Lightning Out", "operation": "generate", "expect": "?"}, {"id": "page_object_missing_object", "page": {"type": "standard__objectPage", "attributes": {"actionName": "list"}}, "host": "Visualforce Lightning Out", "operation": "generate", "expect": "?"}, {"id": "page_object_throwing_action", "page_expr": "({type:'standard__objectPage',attributes:{objectApiName:'Account',get actionName(){throw new TypeError('L14 owned action getter');}}})", "host": "Visualforce Lightning Out", "operation": "generate", "expect": "?"}, {"id": "page_object_throwing_state", "page_expr": "({type:'standard__objectPage',attributes:{objectApiName:'Account',actionName:'list'},get state(){throw new TypeError('L14 owned state getter');}})", "host": "Visualforce Lightning Out", "operation": "generate", "expect": "?"}];
export default class FamilyL14VFControl extends NavigationMixin(LightningElement) {
 @api caseId;
 result = ''; readyId;
 renderedCallback() {
  if (this.readyId === this.caseId || !SPECS.some(s => s.id === this.caseId)) return;
  this.readyId = this.caseId;
  this.publish({id:this.caseId,stage:'ready'});
 }
 publish(value) { this.result = JSON.stringify({host:'Visualforce Lightning Out',api:'59.0',...value}); }
 value(value) { return {type:typeof value,value:value === undefined ? '<undefined>' : value}; }
 error(error) {
  return {name:error == null || error.name === undefined ? typeof error : error.name,
   message:String(error == null || error.message === undefined ? error : error.message),
   code:this.value(error == null ? undefined : error.code)};
 }
 async execute() {
  const spec=SPECS.find(s => s.id === this.caseId);
  if (!spec) return;
  const out={id:spec.id,stage:'done',operation:'GenerateUrl',results:[]};
  const page=(spec.id === "generate_undefined" ? (undefined) : (spec.id === "generate_throwing_type" ? (({get type(){throw new TypeError('L14 owned type getter');},attributes:{}})) : (spec.id === "page_object_throwing_action" ? (({type:'standard__objectPage',attributes:{objectApiName:'Account',get actionName(){throw new TypeError('L14 owned action getter');}}})) : (spec.id === "page_object_throwing_state" ? (({type:'standard__objectPage',attributes:{objectApiName:'Account',actionName:'list'},get state(){throw new TypeError('L14 owned state getter');}})) : spec.page))));
  let pending;
  try { pending=this[NavigationMixin.GenerateUrl](page); }
  catch(error) { out.results.push({phase:'call',error:this.error(error)}); this.publish(out); return; }
  const returnShape={type:typeof pending,thenable:!!pending && typeof pending.then === 'function'};
  try { out.results.push({phase:'resolved',returnShape,url:this.value(await pending)}); }
  catch(error) { out.results.push({phase:'rejected',returnShape,error:this.error(error)}); }
  this.publish(out);
 }
}
