import { LightningElement } from 'lwc'; import { load } from './helper'; export default class Probe extends LightningElement { connectedCallback() { load('c/example'); } }
