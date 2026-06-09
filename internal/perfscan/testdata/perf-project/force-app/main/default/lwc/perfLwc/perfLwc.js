import { LightningElement, wire } from 'lwc';
import uncachedAccounts from '@salesforce/apex/PerfRisk.uncachedAccounts';

export default class PerfLwc extends LightningElement {
    @wire(uncachedAccounts, { ids: '$ids' })
    accounts;
}
