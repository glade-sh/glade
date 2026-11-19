import { LightningElement, api } from 'lwc'; export default class Probe extends LightningElement { @api name; async connectedCallback() { await import(this.name); } }
