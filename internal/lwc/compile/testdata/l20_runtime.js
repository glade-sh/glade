// Owned native L20 application fixture, exported from the native capture.
// __SPECS__ receives input cases only, never captured expected answers.
import { LightningElement, wire } from 'lwc';
import { CurrentPageReference } from 'lightning/navigation';
const SPECS=__SPECS__;
export default class FamilyPresentation extends LightningElement {
 spec; result=''; ready=false; mounted=false; events=[]; data=[]; columns=[];
 items=[]; options=[]; value; sortedBy; sortDirection; draftValues=[];
 @wire(CurrentPageReference) page(ref) {
  if(!ref)return;
  const found=SPECS.find(s=>s.id===(ref.state&&ref.state.c__case));
  if(!found)return;
  this.mounted=false;this.ready=true;this.spec=found;this.events=[];
  this.result=JSON.stringify({id:found.id,stage:'ready'});
 }
 get dt(){return this.mounted&&this.spec.group==='datatable';}
 get tree(){return this.mounted&&this.spec.group==='tree';}
 get grid(){return this.mounted&&this.spec.group==='tree-grid';}
 get dual(){return this.mounted&&this.spec.group==='dual-listbox';}
 get lookup(){return this.mounted&&this.spec.group==='lookup-style-lists';}
 get keyField(){return this.spec.keyField||'id';}
 get maxRowSelection(){return this.spec.maxRowSelection===undefined?1000:this.spec.maxRowSelection;}
 get selectedRows(){return this.spec.selectedRows||[];}
 get disabledRows(){return this.spec.disabledRows||[];}
 get expandedRows(){return this.spec.expandedRows||[];}
 get hideCheckboxColumn(){return !!this.spec.hideCheckboxColumn;}
 get showRowNumberColumn(){return !!this.spec.showRowNumberColumn;}
 get disabled(){return !!this.spec.disabled;}
 get required(){return !!this.spec.required;}
 get requiredOptions(){return this.spec.requiredOptions||[];}
 get min(){return this.spec.min;}
 get max(){return this.spec.max;}
 mount(){
  const s=this.spec;this.data=s.data;this.columns=s.columns;this.items=s.items;
  this.options=s.options;this.value=s.value;this.draftValues=[];
  this.sortedBy=undefined;this.sortDirection=undefined;this.mounted=true;
  this.result=JSON.stringify({id:s.id,stage:'mounted'});
 }
 stable(value){return JSON.parse(JSON.stringify(value===undefined?'<undefined>':value));}
 record(event){
  const d=event.detail||{};let detail;
  if(event.type==='rowselection')detail={selectedRows:(d.selectedRows||[]).map(r=>r.id),config:d.config};
  else if(event.type==='rowaction')detail={action:d.action,row:d.row};
  else if(event.type==='toggle')detail={name:d.name,isExpanded:d.isExpanded,hasChildrenContent:d.hasChildrenContent};
  else detail=d;
  this.events.push({type:event.type,detail:this.stable(detail),bubbles:event.bubbles,composed:event.composed,cancelable:event.cancelable});
  // Prevent tree href navigation; no fixture leaves its owned host.
  if(event.type==='select')event.preventDefault();
  if(event.type==='change'&&d.value!==undefined)this.value=d.value;
 }
 sort(event){
  this.record(event);const d=event.detail;this.sortedBy=d.fieldName;this.sortDirection=d.sortDirection;
  // This is the documented application handler. Capture native sort events and
  // rendered rows; merely assigning sorted-by is never a sort-operation case.
  const direction=d.sortDirection==='asc'?1:-1;
  this.data=[...this.data].sort((a,b)=>{const x=a[d.fieldName],y=b[d.fieldName];
   if(x===y)return 0;if(x==null)return -direction;if(y==null)return direction;
   return (x>y?1:-1)*direction;});
 }
 save(event){
  this.record(event);const drafts=event.detail.draftValues||[];
  this.data=this.data.map(row=>({...row,...(drafts.find(d=>d.id===row.id)||{})}));
  this.draftValues=[];
 }
 method(){
  const control=this.template.querySelector('[data-control]');let observation;
  try{
   if(this.spec.operation==='expandAll'||this.spec.operation==='collapseAll'){
    control[this.spec.operation]();observation={method:this.spec.operation,returned:true};
   }else if(this.spec.operation==='validate'){
    observation={checkValidity:control.checkValidity(),reportValidity:control.reportValidity()};
   }
  }catch(error){observation={name:error.name,message:error.message};}
  this.events.push({type:'method',observation:this.stable(observation)});
 }
 edit(){const control=this.template.querySelector('[data-control]');control.openInlineEdit();}
 finish(){
  const control=this.template.querySelector('[data-control]');let api={};
  try{
   if(control&&typeof control.getSelectedRows==='function')api.selectedRows=control.getSelectedRows().map(r=>r.id);
   if(control&&typeof control.getCurrentExpandedRows==='function')api.expandedRows=control.getCurrentExpandedRows();
   if(this.dual||this.lookup)api.value=control.value;
  }catch(error){api.error={name:error.name,message:error.message};}
  this.result=JSON.stringify({id:this.spec.id,stage:'done',events:this.events,api:this.stable(api),data:this.stable(this.data),value:this.stable(this.value)});
 }
 errorCallback(error){this.result=JSON.stringify({id:this.spec.id,stage:'failed',nativeError:{name:error.name,message:error.message}});}
}
