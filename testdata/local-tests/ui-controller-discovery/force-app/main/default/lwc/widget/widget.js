import { LightningElement, wire } from 'lwc';
import getWidget from '@salesforce/apex/WidgetController.getWidget';
import saveWidget from '@salesforce/apex/WidgetController.saveWidget';
import Save from '@salesforce/label/c.Save';
import RES from '@salesforce/resourceUrl/WidgetAssets';
import ACCOUNT_NAME from '@salesforce/schema/Account.Name';
import { getRecord } from 'lightning/uiRecordApi';
import child from 'c/child';

export default class Widget extends LightningElement {
  accountId;

  @wire(getWidget, { accountId: '$accountId' }) widget;
  @wire(getRecord, { recordId: '$accountId', fields: [ACCOUNT_NAME] }) record;

  save() {
    return saveWidget({ accountId: this.accountId });
  }
}
