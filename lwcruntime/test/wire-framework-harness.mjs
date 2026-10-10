import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import { compileFixture, harnessHTML, repoRoot, startLightningServer } from "./helpers.mjs";

export const wireFrameworkFixture = "lwcruntime/test/fixtures/wire-contract";
export const wireFrameworkFirstRecordId = "001000000000104AAA";
export const wireFrameworkDeniedRecordId = "001000000000104BBB";
const toolchainFallback = path.join(repoRoot, "third_party/lwc");
const componentMeta = path.join(repoRoot, wireFrameworkFixture, "lwc/wireContractProbe/wireContractProbe.js-meta.xml");

function packageVersion(root, name) {
  const packageFile = path.join(root, "node_modules", ...name.split("/"), "package.json");
  assert.ok(fs.existsSync(packageFile), `retained package is missing: ${packageFile}`);
  return JSON.parse(fs.readFileSync(packageFile, "utf8")).version;
}

export async function startWireFrameworkHarness() {
  const toolchainRoot = process.env.GLADE_LWC_TOOLCHAIN_DIR || toolchainFallback;
  assert.equal(packageVersion(toolchainRoot, "@lwc/compiler"), "9.4.3", "unexpected retained compiler version");
  assert.equal(packageVersion(toolchainRoot, "@lwc/engine-dom"), "9.4.3", "unexpected retained engine version");
  assert.ok(fs.existsSync(path.join(toolchainRoot, "node_modules/@lwc/engine-dom/dist/index.js")), "retained engine bundle is missing");
  assert.match(fs.readFileSync(componentMeta, "utf8"), /<apiVersion>\s*67\.0\s*<\/apiVersion>/, "fixture must bind API 67 only");

  const tmpParent = process.env.TMPDIR;
  assert.ok(tmpParent && path.isAbsolute(tmpParent), "guard must provide a bounded absolute TMPDIR");
  const tmpRoot = fs.realpathSync(tmpParent);
  assert.ok(fs.statSync(tmpRoot).isDirectory(), "guard TMPDIR must be a directory");
  const outDir = fs.mkdtempSync(path.join(tmpRoot, "glade-wire-harness-"));
  let server;
  let closed = false;
  const requests = [];

  const close = async () => {
    if (closed) return;
    closed = true;
    try {
      await server?.close();
    } finally {
      fs.rmSync(outDir, { recursive: true, force: true });
    }
  };

  try {
    compileFixture(wireFrameworkFixture, outDir);
    const config = {
      namespace: "c",
      outApps: ["c:lightningout"],
      manifest: {
        modules: {
          "c:wirecontractprobe": {
            url: "/lightning/modules/c/wireContractProbe/wireContractProbe.js",
            tag: "c-wire-contract-probe",
          },
        },
      },
    };
    const moduleScript = `
      import "/lightning/glade.out.js";
      await new Promise((resolve) => {
        window.$Lightning.use("c:lightningOut", () => {
          window.$Lightning.createComponent("c:wireContractProbe", {
            recordId: ${JSON.stringify(wireFrameworkFirstRecordId)},
            deniedRecordId: ${JSON.stringify(wireFrameworkDeniedRecordId)},
          }, "host", resolve);
        });
      });
    `;
    server = await startLightningServer({
      port: 8942,
      compiledDir: outDir,
      gladeOutJS: path.join(repoRoot, "internal/lwcruntime/embed/glade.out.js"),
      pages: {},
      wireHandlers: {
        "/lightning/wire/getRecord": (payload) => {
          const recordId = payload.recordId;
          requests.push({ recordId });
          if (recordId === wireFrameworkDeniedRecordId) {
            return { error: { message: "record access denied", statusCode: 403 } };
          }
          return {
            data: {
              id: recordId,
              fields: { Name: { value: "Wire Canary", displayValue: "Wire Canary" } },
            },
          };
        },
      },
    });
    server.pages["/wire-contract.html"] = harnessHTML(server.baseURL, config, moduleScript);
    Object.defineProperty(server.pages, "/wire-contract-requests.json", {
      enumerable: true,
      get: () => JSON.stringify({ requests }),
    });
    return {
      pageURL: `${server.baseURL}/wire-contract.html`,
      requestLogURL: `${server.baseURL}/wire-contract-requests.json`,
      close,
    };
  } catch (error) {
    await close();
    throw error;
  }
}
