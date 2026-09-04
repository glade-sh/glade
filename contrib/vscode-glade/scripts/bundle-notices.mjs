import { createHash } from "node:crypto";
import { readFileSync, readdirSync, writeFileSync } from "node:fs";
import { relative, resolve, sep } from "node:path";

const root = resolve(import.meta.dirname, "..");
const bundlePath = resolve(root, "out/extension.js");
const metadataPath = resolve(root, "out/extension.meta.json");
const metadata = JSON.parse(readFileSync(metadataPath, "utf8"));
const output = Object.entries(metadata.outputs || {}).find(([path]) => resolve(root, path) === bundlePath)?.[1];
if (!output?.inputs) throw new Error("esbuild metafile does not describe out/extension.js inputs");

function packageRoot(input) {
  const normalized = input.replaceAll("\\", "/");
  const marker = "node_modules/";
  const index = normalized.lastIndexOf(marker);
  if (index < 0) throw new Error(`bundled input is not a package: ${input}`);
  const rest = normalized.slice(index + marker.length).split("/");
  const packageParts = rest[0]?.startsWith("@") ? rest.slice(0, 2) : rest.slice(0, 1);
  if (packageParts.length === 0 || packageParts.some((part) => !part)) throw new Error(`invalid package input: ${input}`);
  return resolve(root, normalized.slice(0, index + marker.length), ...packageParts);
}

const packages = new Map();
for (const [input, details] of Object.entries(output.inputs)) {
  if (!(details?.bytesInOutput > 0) || !input.includes("node_modules/")) continue;
  const directory = packageRoot(input);
  const manifest = JSON.parse(readFileSync(resolve(directory, "package.json"), "utf8"));
  if (!manifest.name || !manifest.version || !manifest.license) throw new Error(`bundled package lacks name, version, or license: ${directory}`);
  const packagePath = relative(root, directory).split(sep).join("/");
  const noticeFiles = readdirSync(directory)
    .filter((file) => /^(license|licence|copying|notice|copyright)([._-].*)?$/i.test(file))
    .sort();
  if (noticeFiles.length === 0) throw new Error(`bundled package lacks a notice file: ${packagePath}`);
  packages.set(packagePath, { name: manifest.name, version: manifest.version, license: manifest.license, packagePath, noticeFiles });
}
if (packages.size === 0) throw new Error("esbuild metafile has no bundled package inputs");

const bundled = [...packages.values()].sort((left, right) => left.packagePath.localeCompare(right.packagePath));
const sha256 = createHash("sha256").update(readFileSync(bundlePath)).digest("hex");
writeFileSync(resolve(root, "out/bundled-dependencies.json"), `${JSON.stringify({
  schemaVersion: 1,
  bundle: { path: "out/extension.js", sha256 },
  packages: bundled,
}, null, 2)}\n`);

const notices = ["Glade VSIX third-party notices", ""];
for (const bundledPackage of bundled) {
  notices.push(`${bundledPackage.name}@${bundledPackage.version}`, `License metadata: ${bundledPackage.license}`, `Package path: ${bundledPackage.packagePath}`, "");
  for (const file of bundledPackage.noticeFiles) {
    notices.push(`--- ${file} ---`, readFileSync(resolve(root, bundledPackage.packagePath, file), "utf8").trimEnd(), "");
  }
}
writeFileSync(resolve(root, "out/THIRD_PARTY_NOTICES.txt"), `${notices.join("\n")}\n`);
