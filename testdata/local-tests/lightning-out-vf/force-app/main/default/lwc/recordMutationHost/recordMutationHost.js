import { LightningElement } from 'lwc';
import { createRecord, updateRecord, deleteRecord } from 'lightning/uiRecordApi';
import ACCOUNT_OBJECT from '@salesforce/schema/Account';
import NAME_FIELD from '@salesforce/schema/Account.Name';

export default class RecordMutationHost extends LightningElement {
    status = 'pending';

    async connectedCallback() {
        try {
            const created = await createRecord({
                apiName: ACCOUNT_OBJECT.objectApiName,
                fields: {
                    [NAME_FIELD.fieldApiName]: 'VF Mutation Account'
                }
            });
            await updateRecord({
                fields: {
                    Id: created.id,
                    [NAME_FIELD.fieldApiName]: 'VF Mutation Updated'
                }
            });
            await deleteRecord(created.id);
            this.status = 'mutation complete';
        } catch (error) {
            this.status = error && error.message ? error.message : 'mutation failed';
        }
    }
}
