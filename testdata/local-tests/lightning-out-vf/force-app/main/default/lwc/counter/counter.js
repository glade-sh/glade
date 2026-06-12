import { LightningElement, api } from 'lwc';

export default class Counter extends LightningElement {
    @api count = 0;

    increase() {
        this.count += 1;
    }
}
