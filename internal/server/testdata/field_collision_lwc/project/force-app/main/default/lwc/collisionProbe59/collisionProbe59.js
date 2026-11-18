import { LightningElement } from 'lwc';
import run from '@salesforce/apex/Parcel62.run';
export default class CollisionProbe extends LightningElement { connectedCallback() { run().then(v => { this.setAttribute('data-capture', v === 'ab' ? 'passed' : 'wrong'); }).catch(() => { this.setAttribute('data-capture', 'error'); }); } }
