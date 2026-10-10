import { LightningElement, api, wire, track } from 'lwc';
import echo from '@salesforce/apex/FamilyL07Oracle.echo';
import { getRecord } from 'lightning/uiRecordApi';
const missing={undefined:true};
function project(v){
 if(v===undefined)return missing;
 if(v===null)return null;
 if(v.error){const e=v.error,b=e.body;return {error:{status:e.status,
  message:b && b.message,exceptionType:b && b.exceptionType,
  bodyArray:Array.isArray(b)?b.map(x=>({message:x.message,errorCode:x.errorCode})):undefined}};}
 if(v.data===undefined)return {data:missing,error:missing};
 if(v.data===null)return {data:null};
 const d=v.data;
 return d.fields?{data:{id:d.id,name:d.fields.Name && d.fields.Name.value}}:{data:d};
}
export default class FamilyL07Consumer67 extends LightningElement {
 marker; ordinal; recordId; @track state={marker:undefined,recordId:undefined};
 connected=0;disconnected=0; history=[]; propertyStates=[]; previous=''; adapter='apex';
 @wire(echo,{marker:'$marker',ordinal:'$ordinal'}) apexProperty;
 @wire(echo,{marker:'$state.marker',ordinal:'$ordinal'}) apexFunction(v){
  this.apexValue=v;this.history.push({adapter:'apex',value:project(v)});
 }
 @wire(getRecord,{recordId:'$recordId',fields:['Account.Name']}) ldsProperty;
 @wire(getRecord,{recordId:'$state.recordId',fields:['Account.Name']}) ldsFunction(v){
  this.ldsValue=v;this.history.push({adapter:'lds',value:project(v)});
 }
 connectedCallback(){this.connected++;}
 disconnectedCallback(){this.disconnected++;}
 get view(){return JSON.stringify(project(this.adapter==='lds'?this.ldsProperty:this.apexProperty));}
 renderedCallback(){
  const signature=this.view;
  if(signature!==this.previous){this.propertyStates.push(JSON.parse(signature));this.previous=signature;}
 }
 @api configure(adapter,marker,ordinal,recordId,mode='replace'){
  this.adapter=adapter;
  this.marker=adapter==='apex'?marker:undefined;this.ordinal=adapter==='apex'?ordinal:undefined;
  this.recordId=adapter==='lds'?recordId:undefined;
  if(mode==='mutate'){this.state.marker=this.marker;this.state.recordId=this.recordId;}
  else this.state={marker:this.marker,recordId:this.recordId};
 }
 @api snapshot(form){
  const value=this.adapter==='lds'?(form==='property'?this.ldsProperty:this.ldsValue):
       (form==='property'?this.apexProperty:this.apexValue);
  return {consumer:form,current:project(value),connected:this.connected,disconnected:this.disconnected,
    functionEmissions:this.history.filter(x=>x.adapter===this.adapter).map(x=>x.value),
    propertyDistinctRenderedStates:this.propertyStates};
 }
 @api mutate(form,operation){
  const v=form==='property'?this.apexProperty:this.apexValue;
  if(!v || !v.data)return {noData:true};
  const d=v.data;let outcome;
  try{
   switch(operation){
    case 'assign':d.marker='changed';break;
    case 'delete':delete d.marker;break;
    case 'nested':d.nested.value='changed';break;
    case 'push':d.items.push(3);break;
    case 'sort':d.items.sort((a,b)=>b-a);break;
    case 'define':Object.defineProperty(d,'marker',{value:'changed'});break;
    case 'clone':{const copy={...d};copy.marker='changed';outcome={copy};break;}
    default:throw new Error('Unknown owned mutation');
   }
   return {returned:true,outcome,beforeAfterData:project(v)};
  }catch(e){return {thrown:{name:e.name,message:e.message},after:project(v)};}
 }
}