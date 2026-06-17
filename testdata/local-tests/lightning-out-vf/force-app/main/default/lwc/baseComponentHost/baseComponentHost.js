import { LightningElement } from 'lwc';

export default class BaseComponentHost extends LightningElement {
    columns = [{ label: 'Name', fieldName: 'name' }];
    rows = [{ id: '1', name: 'VF Local Account' }];
    recordId = '001XX0000000001';
    fields = ['Name'];
    activeTab = '';

    handleTabActive(event) {
        this.activeTab = event.detail && event.detail.value ? event.detail.value : 'details';
    }
}
