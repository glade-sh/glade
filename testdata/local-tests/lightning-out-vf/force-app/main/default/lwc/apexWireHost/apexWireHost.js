import { LightningElement, api, wire } from 'lwc';
import getItems from '@salesforce/apex/ItemCtrl.getItems';

export default class ApexWireHost extends LightningElement {
    @api recordId = '001XX0000000001';

    @wire(getItems, { recordId: '$recordId' })
    items;

    get itemText() {
        return this.items && this.items.data ? this.items.data : '';
    }
}
