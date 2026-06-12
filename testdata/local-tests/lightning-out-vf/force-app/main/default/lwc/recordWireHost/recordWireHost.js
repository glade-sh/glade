import { LightningElement, api, wire } from 'lwc';
import { getRecord } from 'lightning/uiRecordApi';
import ACCOUNT_NAME from '@salesforce/schema/Account.Name';

export default class RecordWireHost extends LightningElement {
    @api recordId;

    @wire(getRecord, { recordId: '$recordId', fields: [ACCOUNT_NAME] })
    record;

    get accountName() {
        const fields = this.record && this.record.data && this.record.data.fields;
        const name = fields && fields.Name;
        return name && name.value ? name.value : '';
    }
}
