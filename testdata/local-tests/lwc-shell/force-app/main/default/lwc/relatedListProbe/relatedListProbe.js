import { LightningElement, api, wire } from 'lwc';
import { getRelatedListRecords } from 'lightning/uiRelatedListApi';

export default class RelatedListProbe extends LightningElement {
    @api parentRecordId = '001000000000001AAA';
    @api relatedListId = 'Contacts';

    @wire(getRelatedListRecords, {
        parentRecordId: '$parentRecordId',
        relatedListId: '$relatedListId',
        fields: ['Contact.LastName']
    })
    contacts;

    get firstName() {
        const records = this.contacts && this.contacts.data && this.contacts.data.records;
        const row = records && records[0];
        return row && row.fields && row.fields.LastName ? row.fields.LastName.value : '';
    }

    get count() {
        return this.contacts && this.contacts.data ? String(this.contacts.data.count) : '';
    }
}
