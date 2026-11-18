import {LightningElement} from 'lwc';
import load45 from '@salesforce/apex/CaptureConvert45.load';
import control45 from '@salesforce/apex/CaptureConvert45.control';
import load54 from '@salesforce/apex/CaptureConvert54.load';
import control54 from '@salesforce/apex/CaptureConvert54.control';
export default class ConvertProbe extends LightningElement {
 rows=[];
 async connectedCallback() {
 let rows=[];
 for (const [version,load,control] of [['45',load45,control45],['54',load54,control54]]) {
  for (const [name,fn,args] of [['control',control,{first:'A'}],['missing',load,{first:'A'}],['null',load,{first:'A',count:null}],['valid',load,{first:'A',count:42}],['invalid',load,{first:'A',count:'not-a-number'}],['object',load,{first:'A',count:{a:1}}]]) {
   try { rows.push([version,name,'ok',await fn(args)]); } catch(e) { rows.push([version,name,'error',e?.body?.message || e?.message || String(e)]); }
  }
 }
 this.rows=rows.map(([version,name,status,output])=>({key:version+name,text:JSON.stringify([version,name,status,output])}));
 }
}
