import { access, readFile } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";

const scriptDir = path.dirname(fileURLToPath(import.meta.url));
const siteRoot = path.resolve(scriptDir, "..");
const docsRoot = path.join(siteRoot, "docs-src");
const configPath = path.join(siteRoot, ".vitepress", "config.ts");

function collectRoutes(config) {
  const routes = new Set();
  const linkPattern = /\blink:\s*['"]([^'"]+)['"]/g;
  for (const match of config.matchAll(linkPattern)) {
    const route = match[1];
    if (!route.startsWith("/") || route.startsWith("//")) continue;
    routes.add(route);
  }
  return [...routes].sort();
}

function candidatePaths(route) {
  const clean = route.split("#", 1)[0].split("?", 1)[0];
  if (clean === "/") return [path.join(docsRoot, "index.md")];
  const relative = clean.replace(/^\/+/, "");
  if (relative.endsWith("/")) {
    const dir = path.join(docsRoot, relative);
    return [path.join(dir, "index.md"), path.join(dir, "README.md")];
  }
  return [
    path.join(docsRoot, `${relative}.md`),
    path.join(docsRoot, relative, "index.md"),
    path.join(docsRoot, relative, "README.md")
  ];
}

async function exists(file) {
  try {
    await access(file);
    return true;
  } catch {
    return false;
  }
}

const config = await readFile(configPath, "utf8");
const missing = [];
for (const route of collectRoutes(config)) {
  const candidates = candidatePaths(route);
  const present = await Promise.all(candidates.map((candidate) => exists(candidate)));
  if (!present.some(Boolean)) {
    missing.push(`${route} -> ${candidates.map((candidate) => path.relative(siteRoot, candidate)).join(" or ")}`);
  }
}

if (missing.length > 0) {
  console.error(`Missing docs route source files:\n${missing.map((item) => `- ${item}`).join("\n")}`);
  process.exit(1);
}
