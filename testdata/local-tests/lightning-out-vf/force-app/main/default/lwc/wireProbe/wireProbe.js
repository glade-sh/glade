import { LightningElement, api, wire } from 'lwc';
import getItems from '@salesforce/apex/ItemCtrl.getItems';

export default class WireProbe extends LightningElement {
    @api recordId = '001XX0000000001';
    imperativeText = '';

    @wire(getItems, { recordId: '$recordId' })
    items;

    connectedCallback() {
        getItems({ recordId: this.recordId }).then((rows) => {
            this.imperativeText = rows && rows.length ? rows[0].Name : '';
        });
    }

    get itemText() {
        const rows = this.items && this.items.data;
        return rows && rows.length ? rows[0].Name : '';
    }
}
