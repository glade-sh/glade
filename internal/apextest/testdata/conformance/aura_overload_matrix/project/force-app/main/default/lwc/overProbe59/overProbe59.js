import {LightningElement} from 'lwc';
import arityA from '@salesforce/apex/CaptureOverA.arity';
import scalarA from '@salesforce/apex/CaptureOverA.scalar';
import namesA from '@salesforce/apex/CaptureOverA.names';
import overlapA from '@salesforce/apex/CaptureOverA.overlap';
import controlA from '@salesforce/apex/CaptureOverA.control';
import arityB from '@salesforce/apex/CaptureOverB.arity';
import scalarB from '@salesforce/apex/CaptureOverB.scalar';
import namesB from '@salesforce/apex/CaptureOverB.names';
import overlapB from '@salesforce/apex/CaptureOverB.overlap';
import controlB from '@salesforce/apex/CaptureOverB.control';
import arityC from '@salesforce/apex/CaptureOverC.arity';
import scalarC from '@salesforce/apex/CaptureOverC.scalar';
import namesC from '@salesforce/apex/CaptureOverC.names';
import overlapC from '@salesforce/apex/CaptureOverC.overlap';
import controlC from '@salesforce/apex/CaptureOverC.control';
export default class OverProbe extends LightningElement {
 result='pending';
 async connectedCallback() {
 let rows=[];
 for (let [name,fn,args] of [
 ['a0',arityA,{}],['a1',arityA,{first:'A'}],['a2',arityA,{first:'A',second:'B'}],['a3',arityA,{first:'A',second:'B',third:'C'}],['ax',arityA,{third:'C',extra:'X'}],
 ['b0',arityB,{}],['b1',arityB,{first:'A'}],['b2',arityB,{first:'A',second:'B'}],['b3',arityB,{first:'A',second:'B',third:'C'}],
 ['c0',arityC,{}],['c3',arityC,{first:'A',second:'B',third:'C'}],
 ['as0',scalarA,{}],['ass',scalarA,{value:'S'}],['asi',scalarA,{value:42}],['asb',scalarA,{value:true}],
 ['bs0',scalarB,{}],['bss',scalarB,{value:'S'}],['bsi',scalarB,{value:42}],
 ['cs0',scalarC,{}],['css',scalarC,{value:'S'}],['csi',scalarC,{value:42}],
 ['an0',namesA,{}],['anr',namesA,{red:'R'}],['anb',namesA,{blue:42}],['anboth',namesA,{red:'R',blue:42}],
 ['bn0',namesB,{}],['bnr',namesB,{red:'R'}],['bnb',namesB,{blue:42}],
 ['cn0',namesC,{}],['cnr',namesC,{red:'R'}],['cnb',namesC,{blue:42}],
 ['ao0',overlapA,{}],['aob',overlapA,{red:'R',blue:42}],['aog',overlapA,{red:'R',green:true}],
 ['bo0',overlapB,{}],['bob',overlapB,{red:'R',blue:42}],['bog',overlapB,{red:'R',green:true}],
 ['co0',overlapC,{}],['cob',overlapC,{red:'R',blue:42}],['cog',overlapC,{red:'R',green:true}],
 ['control',controlA,{item:'ok'}]]) {
 try { rows.push([name,'ok',await fn(args)]); } catch(e) {rows.push([name,'error',e?.body?.message || e?.message || String(e)]);}
 } this.result=JSON.stringify(rows);
 }
}
