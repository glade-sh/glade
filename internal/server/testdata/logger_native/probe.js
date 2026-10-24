import { LightningElement } from 'lwc';
import { log } from 'lightning/logger';
import { observeLogger } from './observations';
export default class TelemetryProbe extends LightningElement {
  observation = 'PENDING';
  connectedCallback() { this.observation = JSON.stringify(observeLogger(log)); }
}
