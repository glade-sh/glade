import { LightningElement, api, wire } from 'lwc';
import { getObjectInfo, getPicklistValues } from 'lightning/uiObjectInfoApi';

export default class ObjectInfoProbe extends LightningElement {
    @api objectApiName = 'Account';
    @api recordTypeId = '012000000000000AAA';
    @api fieldApiName = 'Account.Rating';

    @wire(getObjectInfo, { objectApiName: '$objectApiName' })
    objectInfo;

    @wire(getPicklistValues, { fieldApiName: '$fieldApiName', recordTypeId: '$recordTypeId' })
    picklistValues;

    get objectLabel() {
        return this.objectInfo && this.objectInfo.data ? this.objectInfo.data.label : '';
    }

    get firstPicklistLabel() {
        const values = this.picklistValues && this.picklistValues.data && this.picklistValues.data.values;
        return values && values.length ? values[0].label : '';
    }
}
