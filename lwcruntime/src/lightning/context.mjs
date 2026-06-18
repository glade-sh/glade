export default class LightningContext {
  constructor() {
    this.value = null;
  }
  provide(value) {
    this.value = value;
  }
  consume() {
    return this.value;
  }
}

export function createContextProvider() {
  return () => {};
}
