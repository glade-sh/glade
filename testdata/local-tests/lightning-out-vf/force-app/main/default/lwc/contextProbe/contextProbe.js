import { LightningElement, api } from 'lwc';
import GREETING from '@salesforce/label/c.Greeting';
import WIDGET_ASSETS from '@salesforce/resourceUrl/WidgetAssets';

export default class ContextProbe extends LightningElement {
    @api title = 'VF Context';
    @api recordId;

    greeting = GREETING;
    assetUrl = WIDGET_ASSETS;

    get headingText() {
        return this.title || 'VF Context';
    }
}
