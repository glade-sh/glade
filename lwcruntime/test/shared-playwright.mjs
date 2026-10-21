import { after, afterEach } from "node:test";
import path from "node:path";
import { pathToFileURL } from "node:url";
import { createSharedBrowserSession } from "./shared-browser-session.mjs";

const playwrightPath = process.env.GLADE_LWC_PLAYWRIGHT_MODULE;
const { chromium: playwrightChromium } = await import(
  playwrightPath
    ? pathToFileURL(path.extname(playwrightPath) ? playwrightPath : path.join(playwrightPath, "index.mjs")).href
    : "playwright"
);

const session = createSharedBrowserSession((options) => playwrightChromium.launch(options));

afterEach(() => session.closeContexts());
after(() => session.close());

export const chromium = new Proxy(playwrightChromium, {
  get(target, property) {
    if (property === "launch") return session.launch;
    const value = Reflect.get(target, property, target);
    return typeof value === "function" ? value.bind(target) : value;
  },
});
