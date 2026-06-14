import { LightningElement, api } from 'lwc';
import GREETING from '@salesforce/label/c.LwcProbeGreeting';
import ASSETS from '@salesforce/resourceUrl/LwcProbeAssets';

export default class ContextProbe extends LightningElement {
    @api title = 'Local Shell Context';
    @api recordId;

    greeting = GREETING;
    assetUrl = ASSETS;

    get headingText() {
        return this.title || 'Local Shell Context';
    }
}
