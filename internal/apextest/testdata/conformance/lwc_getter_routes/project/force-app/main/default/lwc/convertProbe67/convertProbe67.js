import {LightningElement} from 'lwc';
import save from '@salesforce/apex/TieSave.save';
import f62direct from '@salesforce/apex/WireCapture62.direct';
import f62finite from '@salesforce/apex/WireCapture62.finite';
import f62recursive from '@salesforce/apex/WireCapture62.recursive';
import f62control from '@salesforce/apex/WireCapture62.control';
import f62reject from '@salesforce/apex/WireCapture62.reject';
import f67direct from '@salesforce/apex/WireCapture67.direct';
import f67finite from '@salesforce/apex/WireCapture67.finite';
import f67recursive from '@salesforce/apex/WireCapture67.recursive';
import f67control from '@salesforce/apex/WireCapture67.control';
import f67reject from '@salesforce/apex/WireCapture67.reject';
export default class ConvertProbe extends LightningElement { result='pending'; async connectedCallback(){let rows=[]; for(let [name,fn] of [['62_direct',f62direct],['62_finite',f62finite],['62_recursive',f62recursive],['62_control',f62control],['62_reject',f62reject],['67_direct',f67direct],['67_finite',f67finite],['67_recursive',f67recursive],['67_control',f67control],['67_reject',f67reject]]) {try {rows.push([name,'ok',JSON.stringify(await fn())]);} catch(e){rows.push([name,'error',e?.body?.message||e?.message||String(e)]);}} await save({label:'Capture67',value:JSON.stringify(rows)});this.result='saved';}}
