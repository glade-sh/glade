export function processImage(input, _options = null) {
  if (!input) {
    return Promise.reject(new Error("Unable to read the input data."));
  }
  return Promise.resolve(input);
}
