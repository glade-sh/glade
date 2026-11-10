import { LightningElement } from 'lwc';
import run from '@salesforce/apex/OuterProbe62.run';
export default class CaptureProbe extends LightningElement { connectedCallback() { run().then(v => { this.setAttribute('data-capture', v === true ? 'passed' : 'wrong'); }).catch(() => { this.setAttribute('data-capture', 'error'); }); } }
