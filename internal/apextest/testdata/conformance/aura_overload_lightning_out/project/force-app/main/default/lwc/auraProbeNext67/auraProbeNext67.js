import { LightningElement } from 'lwc';
import load from '@salesforce/apex/CaptureAura45.load';
import control from '@salesforce/apex/CaptureAura45.control';
import reverse from '@salesforce/apex/CaptureAuraReverse45.load';
import choose from '@salesforce/apex/CaptureAuraReverse45.choose';
export default class AuraProbe extends LightningElement {
 result = 'pending';
 async connectedCallback() {
  let rows=[];
  for (const [name, fn, args] of [['control', control, {item:'ok'}], ['empty',load,{}], ['one',load,{recordId:'A'}], ['two',load,{recordId:'A',groupId:'B'}], ['groupOnly',load,{groupId:'B'}], ['extra',load,{recordId:'A',groupId:'B',extra:'C'}], ['reverse-empty',reverse,{}], ['reverse-one',reverse,{recordId:'A'}], ['reverse-two',reverse,{recordId:'A',groupId:'B'}], ['choose-empty',choose,{}], ['choose-red',choose,{red:'R'}], ['choose-blue',choose,{blue:42}], ['choose-both',choose,{red:'R',blue:42}]]) {
   try { rows.push([name,'success',await fn(args)]); } catch(e) { rows.push([name,'error',e?.body?.message || e?.message || String(e)]); }
  }
  this.result=JSON.stringify(rows);
 }
}