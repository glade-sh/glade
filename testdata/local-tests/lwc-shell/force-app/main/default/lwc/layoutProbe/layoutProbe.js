import { LightningElement, api, wire } from 'lwc';
import { getLayout } from 'lightning/uiLayoutApi';

export default class LayoutProbe extends LightningElement {
    @api objectApiName = 'Account';
    @api recordTypeId = '012000000000000AAA';
    @api layoutType = 'Full';
    @api mode = 'Create';
    @api formFactor = 'Large';

    @wire(getLayout, {
        objectApiName: '$objectApiName',
        recordTypeId: '$recordTypeId',
        layoutType: '$layoutType',
        mode: '$mode',
        formFactor: '$formFactor'
    })
    layout;

    get heading() {
        const sections = this.layout && this.layout.data && this.layout.data.sections;
        return sections && sections.length ? sections[0].heading : '';
    }

    get firstField() {
        const sections = this.layout && this.layout.data && this.layout.data.sections;
        const row = sections && sections[0] && sections[0].layoutRows && sections[0].layoutRows[0];
        const item = row && row.layoutItems && row.layoutItems[0];
        return item ? item.fieldApiName : '';
    }

    get modeLabel() {
        return this.layout && this.layout.data ? this.layout.data.mode : '';
    }
}
