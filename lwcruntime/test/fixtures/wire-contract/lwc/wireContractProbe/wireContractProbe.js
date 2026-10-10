import { LightningElement, api, wire } from "lwc";
import { getRecord } from "lightning/uiRecordApi";
import ACCOUNT_NAME from "@salesforce/schema/Account.Name";

export default class WireContractProbe extends LightningElement {
  @api recordId;
  @api deniedRecordId;

  status = "waiting";
  activeRecordId;
  delivered = undefined;
  deliveries = [];
  mutationBefore = "not-attempted";
  mutationAfter = "not-attempted";
  mutationThrew = "not-attempted";

  connectedCallback() {
    this.activeRecordId = this.recordId;
  }

  @wire(getRecord, { recordId: "$activeRecordId", fields: [ACCOUNT_NAME] })
  receiveRecord(value) {
    const data = value?.data;
    const error = value?.error;
    this.deliveries = [
      ...this.deliveries,
      { kind: data !== undefined ? "data" : error !== undefined ? "error" : "empty" },
    ];
    if (data !== undefined) {
      this.delivered = data;
      this.status = `data:${data.fields.Name.value}`;
    } else if (error !== undefined) {
      this.delivered = undefined;
      this.status = `error:${error.message}`;
    }
  }

  get recordName() {
    return this.delivered?.fields?.Name?.value ?? "";
  }

  get deliveryHistoryRows() {
    return this.deliveries.map((delivery, index) => ({
      key: String(index),
      text: `${index}:${delivery.kind}`,
    }));
  }

  handleMutationAttempt() {
    const field = this.delivered?.fields?.Name;
    const before = field?.value ?? "unavailable";
    let threw = false;
    try {
      field.value = "mutation-probe";
    } catch (_error) {
      threw = true;
    }
    this.mutationBefore = before;
    this.mutationAfter = field?.value ?? "unavailable";
    this.mutationThrew = String(threw);
  }

  handleDeniedRequest() {
    this.activeRecordId = this.deniedRecordId;
  }
}
