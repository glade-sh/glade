import { LightningElement } from 'lwc';
import load from '@salesforce/apex/CaptureAura45.load';
import control from '@salesforce/apex/CaptureAura45.control';
export default class AuraProbe extends LightningElement {
 result = 'pending';
 async connectedCallback() {
  let rows=[];
  for (const [name, fn, args] of [['control', control, {item:'ok'}], ['empty',load,{}], ['one',load,{recordId:'A'}], ['two',load,{recordId:'A',groupId:'B'}], ['groupOnly',load,{groupId:'B'}], ['extra',load,{recordId:'A',groupId:'B',extra:'C'}]]) {
   try { rows.push([name,'success',await fn(args)]); } catch(e) { rows.push([name,'error',e?.body?.message || e?.message || String(e)]); }
  }
  this.result=JSON.stringify(rows);
 }
}