import { LightningElement } from 'lwc';
import { NavigationMixin } from 'lightning/navigation';
export default class Probe extends NavigationMixin(LightningElement) {
  async urls(recordId) {
    const names = ['Account', 'Record', 'Contact', 'NoSuchObject__c'];
    return Promise.all(names.map(objectApiName => this[NavigationMixin.GenerateUrl]({
      type: 'standard__recordPage', attributes: {recordId, objectApiName, actionName: 'view'}
    })));
  }
}
