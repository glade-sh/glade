import { LightningElement } from 'lwc';

export default class BaseComponentHost extends LightningElement {
    columns = [{ label: 'Name', fieldName: 'name' }];
    rows = [{ id: '1', name: 'VF Local Account' }];
}
