#!/usr/bin/env node
import { execFile as execFileCallback } from "node:child_process";
import { mkdir, rm, writeFile } from "node:fs/promises";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { promisify } from "node:util";

const siteRoot = resolve(dirname(fileURLToPath(import.meta.url)), "../..");
const repoRoot = resolve(siteRoot, "..");
const execFile = promisify(execFileCallback);
const configuredOutRoot = process.env.HELP_PROJECT_ROOT?.trim();
const outRoot = configuredOutRoot
  ? resolve(configuredOutRoot)
  : resolve(repoRoot, ".glade/macrodata-apex");
const refinementServicePath = resolve(outRoot, "force-app/main/default/classes/RefinementService.cls");
const refinementServiceSource = `public with sharing class RefinementService {
    public static Account openFile(String name) {
        Account account = new Account(Name = name);
        insert account;
        return account;
    }

    public static Integer refinedCount() {
        return [SELECT Id FROM Account].size();
    }
}
`;

async function git(...args) {
  await execFile("git", ["-C", outRoot, ...args]);
}

await rm(outRoot, { recursive: true, force: true });
await mkdir(resolve(outRoot, "force-app/main/default/classes"), { recursive: true });
await mkdir(resolve(outRoot, ".github/workflows"), { recursive: true });
await mkdir(resolve(outRoot, "data"), { recursive: true });
await mkdir(resolve(outRoot, "reports"), { recursive: true });

await writeFile(resolve(outRoot, "sfdx-project.json"), JSON.stringify({
  packageDirectories: [{ path: "force-app", default: true }],
  name: "macrodata-apex",
  namespace: "",
  sourceApiVersion: "65.0"
}, null, 2) + "\n");

await writeFile(resolve(outRoot, "glade.yml"), `project:
  root: .
  packageDirs: [force-app]
  managedPackageDependencies: []
org:
  features: []
`);

await writeFile(refinementServicePath, refinementServiceSource);

await writeFile(resolve(outRoot, "force-app/main/default/classes/RefinementServiceTest.cls"), `@IsTest
private class RefinementServiceTest {
    @IsTest
    static void opensFile() {
        Account account = RefinementService.openFile('Quarterly refinement file');
        System.assertNotEquals(null, account.Id);
        System.assertEquals(1, RefinementService.refinedCount());
    }
}
`);

await writeFile(resolve(outRoot, "anonymous.apex"), `Account account = RefinementService.openFile(
    'Walkthrough refinement file'
);
System.debug(account.Id);
System.debug('Refined files: ' + RefinementService.refinedCount());
`);

await writeFile(resolve(outRoot, "reports/anonymous-output.txt"), `64.0 APEX_CODE,DEBUG,APEX_PROFILING,INFO,CALLOUT,DB
00:00:00.000 (0)|USER_INFO|[EXTERNAL]|005000000000001|help@example.com
00:00:00.001 (1000000)|EXECUTION_STARTED
00:00:00.002 (2000000)|CODE_UNIT_STARTED|[EXTERNAL]|execute_anonymous_apex
00:00:00.003 (3000000)|DML_BEGIN|[1]|Op:Insert|Type:Account|Rows:1
00:00:00.004 (4000000)|DML_END|[1]
00:00:00.005 (5000000)|USER_DEBUG|[2]|DEBUG|001000000000001
00:00:00.006 (6000000)|USER_DEBUG|[3]|DEBUG|Refined files: 1
00:00:00.007 (7000000)|CODE_UNIT_FINISHED|execute_anonymous_apex
00:00:00.008 (8000000)|EXECUTION_FINISHED
`);

await writeFile(resolve(outRoot, ".github/workflows/glade.yml"), `name: glade
on: [pull_request]

jobs:
  glade:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0
      - run: curl -fsSL https://glade.sh/install.sh | env GLADE_VERSION=v0.2.15 sh
      - run: echo "$HOME/.local/bin" >> "$GITHUB_PATH"
      - run: glade doctor
      - run: mkdir -p reports
      - run: glade check --project . --format sarif --output reports/glade-check.sarif --no-progress
      - run: glade test changed --project . --since origin/main --json --no-progress > reports/glade-test-changed.json
      - run: glade test --project . --junit reports/glade-junit.xml --no-progress
      - uses: actions/upload-artifact@v4
        if: always()
        with:
          name: glade-results
          path: |
            reports/glade-check.sarif
            reports/glade-test-changed.json
            reports/glade-junit.xml
`);

await writeFile(resolve(outRoot, "seed.json"), JSON.stringify({
  version: "glade.storage.v1",
  objects: [
    {
      name: "Account",
      records: [
        {
          alias: "seed-account",
          fields: {
            Name: { kind: "string", string: "Seed refinement file" }
          }
        }
      ]
    }
  ]
}, null, 2) + "\n");

await writeFile(resolve(outRoot, "data/insertOrder.json"), JSON.stringify([
  {
    sobject: "Account",
    files: ["accounts.json"]
  },
  {
    sobject: "Contact",
    files: ["contacts.json"]
  }
], null, 2) + "\n");

await writeFile(resolve(outRoot, "data/accounts.json"), JSON.stringify({
  records: [
    {
      attributes: { type: "Account", referenceId: "AccountRef1" },
      Name: "Imported refinement file"
    }
  ]
}, null, 2) + "\n");

await writeFile(resolve(outRoot, "data/contacts.json"), JSON.stringify({
  records: [
    {
      attributes: { type: "Contact", referenceId: "ContactRef1" },
      LastName: "Refiner",
      AccountId: "@AccountRef1"
    }
  ]
}, null, 2) + "\n");

await git("init", "-q");
await git("config", "user.name", "Glade Help");
await git("config", "user.email", "help@example.com");
await git("config", "commit.gpgsign", "false");
await git("add", ".");
await git("commit", "-qm", "Seed help walkthrough project");

console.log(outRoot);
