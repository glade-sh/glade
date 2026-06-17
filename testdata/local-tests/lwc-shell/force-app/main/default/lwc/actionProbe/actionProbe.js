import { LightningElement, api } from 'lwc';

export default class ActionProbe extends LightningElement {
    @api title = 'Action Probe';
    @api recordId;
    @api objectApiName;
    @api actionName;
    @api actionType;
    @api state;
    @api pageReference;

    get headingText() {
        return this.title || 'Action Probe';
    }
}
