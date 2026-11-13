import { LightningElement } from 'lwc';

export default class GladeClassProbe extends LightningElement {
    options = [
        { label: 'Alpha', value: 'a' },
        { label: 'Beta', value: 'b' }
    ];
    on = true;
    off = false;
    reported = false;

    renderedCallback() {
        if (this.reported) {
            return;
        }
        this.reported = true;
        this.template.querySelectorAll('[data-report]').forEach((element) => element.reportValidity());
    }
}
