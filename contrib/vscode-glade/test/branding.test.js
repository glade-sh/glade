const assert = require("assert");
const fs = require("fs");
const path = require("path");

const root = path.resolve(__dirname, "..");
const activityIcon = fs.readFileSync(path.join(root, "media/glade.svg"), "utf8");
const brandMarkPath = path.join(root, "media/glade-brand.svg");

assert(fs.existsSync(brandMarkPath), "webview brand mark asset must exist");

const brandMark = fs.readFileSync(brandMarkPath, "utf8");
for (const color of ["#9BE870", "#B7FF8A"]) {
  assert(brandMark.includes(color), `brand mark must include ${color}`);
}

assert(!activityIcon.includes("#C5C5C5"), "activity icon must not use the old gray placeholder stroke");
assert(!activityIcon.includes("#9BE870"), "activity icon must be themeable in the Activity Bar");
assert(!activityIcon.includes("#B7FF8A"), "activity icon border must be themeable in the Activity Bar");
assert(
  activityIcon.includes("currentColor"),
  "activity icon must use currentColor for VS Code Activity Bar states",
);
assert(activityIcon.includes("viewBox=\"0 0 500 500\""), "activity icon must use the site contour mark geometry");
