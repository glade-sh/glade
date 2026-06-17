import { LightningElement, wire } from 'lwc';
import basePath from '@salesforce/community/basePath';
import communityId from '@salesforce/community/Id';
import siteId from '@salesforce/site/Id';
import isGuest from '@salesforce/user/isGuest';
import { CurrentPageReference, NavigationMixin } from 'lightning/navigation';

export default class CommunityProbe extends NavigationMixin(LightningElement) {
    @wire(CurrentPageReference) pageReference;

    generatedUrl = '';

    connectedCallback() {
        this[NavigationMixin.GenerateUrl]({
            type: 'comm__namedPage',
            attributes: { name: 'Account' },
            state: { c__view: 'summary' },
        }).then((url) => {
            this.generatedUrl = url;
        });
    }

    get basePath() {
        return basePath;
    }

    get communityId() {
        return communityId;
    }

    get siteId() {
        return siteId;
    }

    get guestText() {
        return String(isGuest);
    }

    get pageReferenceType() {
        return this.pageReference?.type || '';
    }
}
