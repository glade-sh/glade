import { LightningElement } from 'lwc';
import GREETING from '@salesforce/label/c.Greeting';
import WIDGET_ASSETS from '@salesforce/resourceUrl/WidgetAssets';

export default class LabelResourceHost extends LightningElement {
    greeting = GREETING;
    assetUrl = WIDGET_ASSETS;
}
