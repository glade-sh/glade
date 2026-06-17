import { LightningElement, wire } from 'lwc';
import { CurrentPageReference, NavigationMixin } from 'lightning/navigation';
import { ShowToastEvent } from 'lightning/platformShowToastEvent';
import { createMessageContext, publish, subscribe, unsubscribe } from 'lightning/messageService';
import { loadScript, loadStyle } from 'lightning/platformResourceLoader';
import SERVICE_SCRIPT from '@salesforce/resourceUrl/ServiceScript';
import SERVICE_STYLES from '@salesforce/resourceUrl/ServiceStyles';
import LWC_PROBE from '@salesforce/messageChannel/LwcProbe__c';

export default class ServiceHost extends NavigationMixin(LightningElement) {
    @wire(CurrentPageReference) pageReference;

    toastTitle = '';
    messageRecord = '';
    resourceStatus = 'pending';
    navError = '';
    subscription;
    messageContext = createMessageContext();

    async connectedCallback() {
        this.subscription = subscribe(this.messageContext, LWC_PROBE, (message) => {
            this.messageRecord = message && message.recordId ? message.recordId : '';
        });
        publish(this.messageContext, LWC_PROBE, { recordId: '001XX0000000001' });
        this.dispatchEvent(new ShowToastEvent({
            title: 'VF Toast',
            message: 'Visualforce service toast',
            variant: 'success'
        }));
        this.toastTitle = 'VF Toast';
        try {
            await Promise.all([
                loadScript(this, SERVICE_SCRIPT),
                loadStyle(this, SERVICE_STYLES)
            ]);
            this.resourceStatus = window.__vfServiceScriptLoaded ? 'loaded' : 'missing';
        } catch (error) {
            this.resourceStatus = error && error.message ? error.message : 'failed';
        }
        try {
            await this[NavigationMixin.GenerateUrl]({
                type: 'standard__objectPage',
                attributes: {
                    objectApiName: 'Account',
                    actionName: 'new'
                }
            });
        } catch (error) {
            this.navError = error && error.code ? error.code : 'missing';
        }
    }

    disconnectedCallback() {
        unsubscribe(this.subscription);
    }

    get pageType() {
        return this.pageReference && this.pageReference.type ? this.pageReference.type : '';
    }
}
