export const DEFAULT_CONFIG = {
  navKey: "F6",
  f6RegionAttribute: "data-f6-region",
  f6RegionHighlightClass: "f6-highlight",
};

export const getActiveElement = (element) => element?.getRootNode?.().activeElement || document.activeElement;

export class F6Controller {
  constructor(config = DEFAULT_CONFIG) {
    this.config = config;
  }
  initialize() {}
  disable() {}
  enable() {}
  disconnect() {}
}

export const createF6Controller = () => new F6Controller();
export const getCurrentRegionAttributeName = () => DEFAULT_CONFIG.f6RegionAttribute;
export default F6Controller;
