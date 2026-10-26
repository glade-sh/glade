package lwcbrowser

// Desktop observations at source API 62 and 67 are pinned in
// testdata/mobile_capabilities/org.tsv. Hardware scanning requires a mobile host.
func mobileCapabilitiesModuleJS() string {
	return `function unavailableMessage(method) {
  return "Cannot read properties of undefined (reading '" + method + "')";
}
function unavailableScan(method) {
  return Promise.reject({ code: "UNKNOWN_REASON", message: unavailableMessage(method) });
}
export function getBarcodeScanner() {
  return {
    isAvailable() { return false; },
    barcodeTypes: {
      CODE_128: "code128",
      CODE_39: "code39",
      CODE_93: "code93",
      DATA_MATRIX: "datamatrix",
      EAN_13: "ean13",
      EAN_8: "ean8",
      ITF: "itf",
      UPC_A: "upca",
      UPC_E: "upce",
      PDF_417: "pdf417",
      QR: "qr",
    },
    scan() { return unavailableScan("scan"); },
    beginCapture() { return unavailableScan("beginCapture"); },
    resumeCapture() { return unavailableScan("resumeCapture"); },
    dismiss() { throw new TypeError(unavailableMessage("dismiss")); },
    endCapture() { throw new TypeError(unavailableMessage("endCapture")); },
  };
}
`
}
