import { recordToast, rememberToastEvent } from "../shell/toast-service.mjs";

export { recordToast };
export const SHOW_TOAST_EVENT_NAME = "lightning__showtoast";
export class ShowToastEvent extends CustomEvent {
  constructor(detail = {}) {
    const options = { ...detail };
    super(SHOW_TOAST_EVENT_NAME, {
      bubbles: true, composed: true, cancelable: true,
      detail: { label: options.title ?? "" },
    });
    // The public detail stays { label }; mode, message and variant travel by
    // event identity so the local toast UI can honor them.
    rememberToastEvent(this, options);
  }
}
export default ShowToastEvent;
