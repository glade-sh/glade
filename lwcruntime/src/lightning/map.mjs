import {
  LightningElement,
  freezeTemplate,
  registerComponent,
  registerDecorators,
  registerTemplate,
} from "lwc";
import { renderMap as renderMarkerList } from "./base.mjs";

// Only the iframe boundary is observed; hosted tiles and geocoding stay outside
// the local renderer. Keep the existing marker list for other marker inputs.
function renderMapFrame($api, $cmp) {
  return [$api.h("iframe", { key: 0, attrs: { title: $cmp.frameTitle } }, [])];
}
renderMapFrame.stylesheets = [];
const frameTemplate = registerTemplate(renderMapFrame);
freezeTemplate(frameTemplate);
class MapFrame extends LightningElement {}
registerDecorators(MapFrame, { publicProps: { frameTitle: { config: 0 } } });
const PrimitiveIframe = registerComponent(MapFrame, {
  tmpl: frameTemplate,
  sel: "lightning-primitive-iframe",
  apiVersion: 63,
});

function invalidCoordinateMarker(marker) {
  const location = marker?.location;
  if (!location || typeof location !== "object") return false;
  const invalid = coordinate => (typeof coordinate === "number" || typeof coordinate === "string") &&
    !Number.isFinite(Number(coordinate));
  return invalid(location.Latitude) || invalid(location.Longitude);
}

function renderMap($api, $cmp) {
  if (!$cmp.isFrameBoundary) return renderMarkerList($api, $cmp);
  return [$api.h("div", { key: 0, className: "slds-map_container" }, [
    $api.h("div", { key: 1, className: "slds-map" }, [
      $api.c("lightning-primitive-iframe", PrimitiveIframe, {
        key: 2,
        props: {
          frameTitle: $cmp.mapMarkers.length ? "Map of " + ($cmp.markersTitle || "") : "Map Container",
        },
      }, []),
    ]),
  ])];
}
renderMap.stylesheets = [];
const template = registerTemplate(renderMap);
freezeTemplate(template);

class LightningMap extends LightningElement {
  constructor() {
    super();
    this._mapMarkers = [];
  }

  get mapMarkers() {
    return this._mapMarkers;
  }

  set mapMarkers(value) {
    this._mapMarkers = Array.isArray(value) ? value.map(marker => {
      if (marker == null || typeof marker !== "object" || Array.isArray(marker)) return marker;
      return marker.description === undefined ? { ...marker, description: "" } : marker;
    }) : [];
    this.updateBoundaryClass();
  }

  get isFrameBoundary() {
    return !this.mapMarkers.length || this.mapMarkers.every(invalidCoordinateMarker);
  }

  connectedCallback() {
    this.updateBoundaryClass();
  }

  updateBoundaryClass() {
    if (this.isConnected) this.classList.toggle("slds-grid", this.isFrameBoundary);
  }
}
registerDecorators(LightningMap, {
  publicProps: {
    mapMarkers: { config: 3 },
    listView: { config: 0 },
    zoomLevel: { config: 0 },
    markersTitle: { config: 0 },
    markers: { config: 0 },
    items: { config: 0 },
  },
  fields: ["_mapMarkers"],
});
export default registerComponent(LightningMap, {
  tmpl: template,
  sel: "lightning-map",
  apiVersion: 63,
});
