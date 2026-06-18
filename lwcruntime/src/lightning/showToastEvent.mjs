import { recordToast } from "../shell/toast-service.mjs";

export { recordToast };
export const SHOW_TOAST_EVENT_NAME = "lightning__showtoast";
export class ShowToastEvent extends CustomEvent {
  constructor(detail = {}) {
    super(SHOW_TOAST_EVENT_NAME, { bubbles: true, composed: true, cancelable: true, detail });
  }
}
export default ShowToastEvent;
