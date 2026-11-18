import save from '@salesforce/apex/OvSave.save';
import ctl from '@salesforce/apex/OvCtl.ctl';
import {LightningElement} from 'lwc';
import fOvAa45S from '@salesforce/apex/OvAa45S.pick';
import fOvZz45S from '@salesforce/apex/OvZz45S.pick';
import fOvAa45I from '@salesforce/apex/OvAa45I.pick';
import fOvZz45I from '@salesforce/apex/OvZz45I.pick';
import fOvAa54S from '@salesforce/apex/OvAa54S.pick';
import fOvZz54S from '@salesforce/apex/OvZz54S.pick';
import fOvAa54I from '@salesforce/apex/OvAa54I.pick';
import fOvZz54I from '@salesforce/apex/OvZz54I.pick';
export default class OvProbe extends LightningElement { result='pending'; async connectedCallback(){let rows=[];
const cases=[['missing',{}],['null',{value:null}],['numeric',{value:42}],['string',{value:'bad'}],['bool',{value:true}],['decimal',{value:1.5}]];
for (const [name,fn] of [['ctl',ctl],['OvAa45S',fOvAa45S],['OvZz45S',fOvZz45S],['OvAa45I',fOvAa45I],['OvZz45I',fOvZz45I],['OvAa54S',fOvAa54S],['OvZz54S',fOvZz54S],['OvAa54I',fOvAa54I],['OvZz54I',fOvZz54I]]) { for (let rep=0; rep<3; rep++) { for (const [c,args] of cases) { try { rows.push([name,c,rep,'ok',await fn(args)]); } catch(e) { rows.push([name,c,rep,'error',e?.body?.message||e?.message||String(e)]); } } } }
for (let i=0;i<rows.length;i+=40) { await save({label:'Ov59_'+i,value:JSON.stringify(rows.slice(i,i+40))}); } this.result='saved'; }}
