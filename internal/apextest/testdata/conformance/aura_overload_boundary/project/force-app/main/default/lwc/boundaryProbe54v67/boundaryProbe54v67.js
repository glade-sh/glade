import { LightningElement } from 'lwc';
import load from '@salesforce/apex/BoundaryAction54.load';
import control from '@salesforce/apex/BoundaryAction54.control';
export default class BoundaryProbe54v67 extends LightningElement {
 result='pending'; async connectedCallback() { let rows=[]; for (const [name,fn,args] of [['control',control,{value:'ok'}],['empty',load,{}],['one',load,{first:'A'}],['both',load,{first:'A',second:'B'}],['extra',load,{second:'B',extra:'C'}]]) {try { rows.push([name,'success',await fn(args)]); } catch(e) { rows.push([name,'error',e?.body?.message || e?.message || String(e)]); } } this.result=JSON.stringify(rows); }
}
