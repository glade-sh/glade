import { LightningElement, api, wire } from 'lwc';
import wireAccounts from '@salesforce/apex/LwcProbeController.wireAccounts';
import imperativeAccount from '@salesforce/apex/LwcProbeController.imperativeAccount';

export default class WireProbe extends LightningElement {
    @api industry = 'Technology';
    imperativeName = '';

    @wire(wireAccounts, { industry: '$industry' })
    accounts;

    get firstWiredName() {
        const rows = this.accounts && this.accounts.data;
        return rows && rows.length ? rows[0].Name : '';
    }

    async loadImperative() {
        const account = await imperativeAccount({ name: 'Imperative Shell Account' });
        this.imperativeName = account && account.Name ? account.Name : '';
    }
}
