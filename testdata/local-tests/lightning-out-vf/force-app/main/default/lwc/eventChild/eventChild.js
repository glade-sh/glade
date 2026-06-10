import { LightningElement } from 'lwc';

export default class EventChild extends LightningElement {
    handleClick() {
        this.dispatchEvent(
            new CustomEvent('select', {
                detail: { id: '1' },
                bubbles: true,
                composed: true,
            })
        );
    }
}
