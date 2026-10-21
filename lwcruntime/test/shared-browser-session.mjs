function stableKey(value) {
  if (value === undefined) return "undefined";
  if (value === null || typeof value !== "object") return JSON.stringify(value);
  if (Array.isArray(value)) return `[${value.map(stableKey).join(",")}]`;
  return `{${Object.keys(value).sort().map((key) => `${JSON.stringify(key)}:${stableKey(value[key])}`).join(",")}}`;
}

export function createSharedBrowserSession(launchBrowser) {
  let launchPromise;
  let optionsKey;
  let shutdownBrowser;
  let closed = false;

  async function closeContexts() {
    if (!launchPromise) return;
    const browser = await launchPromise.catch(() => undefined);
    if (!browser) return;
    await Promise.all(browser.contexts().map((context) => context.close()));
  }

  async function launch(options = {}) {
    const requestedKey = stableKey(options);
    if (launchPromise) {
      if (requestedKey !== optionsKey) {
        throw new Error("a test file must use consistent Playwright launch options");
      }
      return launchPromise;
    }

    optionsKey = requestedKey;
    launchPromise = Promise.resolve()
      .then(() => launchBrowser(options))
      .then((browser) => {
        shutdownBrowser = browser.close.bind(browser);
        return new Proxy(browser, {
          get(target, property) {
            if (property === "close") return closeContexts;
            const value = Reflect.get(target, property, target);
            return typeof value === "function" ? value.bind(target) : value;
          },
        });
      })
      .catch((error) => {
        launchPromise = undefined;
        optionsKey = undefined;
        throw error;
      });
    return launchPromise;
  }

  async function close() {
    const pending = launchPromise;
    if (!pending) return;
    launchPromise = undefined;
    optionsKey = undefined;
    const browser = await pending.catch(() => undefined);
    if (!browser || closed) return;
    closed = true;
    await shutdownBrowser();
  }

  return { launch, closeContexts, close };
}
