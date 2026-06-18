#!/usr/bin/env node
import fs from "node:fs";
import path from "node:path";

const corpusRoot = process.env.GLADE_LWC_CORPUS || "/Users/matt/.sf-repo-analysis/repos";
const repos = [
  "src-nbm-solhub-develop",
  "src-nmb-namz-prog-develop",
  "src-nmb-nc-develop",
  "src-nmb-nu-develop",
  "src-nmb-nudev-develop",
  "src-nmb-nuq-develop",
  "src-nmb-nutpl-develop",
  "src-nmb-nutplx-master",
  "sf-cred-pkg-develop",
];

function walk(dir, out = []) {
  for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
    if ([".git", ".sf", ".sfdx", "node_modules"].includes(entry.name)) {
      continue;
    }
    const full = path.join(dir, entry.name);
    if (entry.isDirectory()) {
      walk(full, out);
    } else {
      out.push(full);
    }
  }
  return out;
}

function count(files, pattern) {
  return files.filter((file) => pattern.test(path.relative(corpusRoot, file).split(path.sep).join("/"))).length;
}

const result = {};
for (const repo of repos) {
  const root = path.join(corpusRoot, repo);
  const files = fs.existsSync(root) ? walk(root) : [];
  result[repo] = {
    root,
    exists: fs.existsSync(root),
    files: files.length,
    lwcFiles: count(files, /\/lwc\/[^/]+\/[^/]+\.(js|html|css|js-meta\.xml)$/),
    apexClasses: count(files, /\.cls$/),
    staticResources: count(files, /\/staticresources\//),
  };
}

console.log(JSON.stringify(result, null, 2));
