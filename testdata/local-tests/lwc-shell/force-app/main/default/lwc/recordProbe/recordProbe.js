import { LightningElement, api, wire } from 'lwc';
import { getRecord } from 'lightning/uiRecordApi';
import ACCOUNT_NAME from '@salesforce/schema/Account.Name';
import ACCOUNT_INDUSTRY from '@salesforce/schema/Account.Industry';

const FIELDS = [ACCOUNT_NAME, ACCOUNT_INDUSTRY];

export default class RecordProbe extends LightningElement {
    @api recordId;

    @wire(getRecord, { recordId: '$recordId', fields: FIELDS })
    record;

    get accountName() {
        const fields = this.record && this.record.data && this.record.data.fields;
        const name = fields && fields.Name;
        return name && name.value ? name.value : '';
    }

    get industry() {
        const fields = this.record && this.record.data && this.record.data.fields;
        const industry = fields && fields.Industry;
        return industry && industry.value ? industry.value : '';
    }
}
