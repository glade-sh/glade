import { LightningElement } from 'lwc';
import { getBarcodeScanner } from 'lightning/mobileCapabilities';
export default class DeviceProbe extends LightningElement {
    result = 'pending';
    async connectedCallback() {
        const scanner = getBarcodeScanner();
        const result = { factory: typeof getBarcodeScanner, available: scanner.isAvailable(), barcodeTypes: scanner.barcodeTypes };
        for (const method of ['scan', 'dismiss', 'beginCapture', 'resumeCapture', 'endCapture']) {
            let kind = 'throw';
            try {
                const returned = scanner[method]({ barcodeTypes: [scanner.barcodeTypes.QR] });
                kind = returned && typeof returned.then === 'function' ? 'promise' : 'sync';
                const value = await returned;
                result[method] = { kind, value: value === undefined ? 'undefined' : value };
            } catch (error) {
                result[method] = { kind, code: error.code, message: error.message, name: error.name };
            }
        }
        this.result = JSON.stringify(result);
    }
}
