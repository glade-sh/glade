import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { existsSync } from "node:fs";
import { mkdir, mkdtemp, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { fileURLToPath } from "node:url";
import { test } from "node:test";

const repo = fileURLToPath(new URL("../../", import.meta.url));
for (const script of ["site/install.sh", "internal/gladecli/update_installer.sh"]) test(`${script} rejects an unavailable install lock`, async () => {
  const root = await mkdtemp(join(tmpdir(), "glade-denied-lock-"));
  const commands = join(root, "commands");
  await mkdir(commands);
  await writeFile(join(commands, "sitecustomize.py"), `import errno,os\n_orig=os.open\ndef guarded(path,*args,**kwargs):\n if os.fsdecode(path).endswith('/.install.lock'):\n  raise PermissionError(errno.EACCES, 'permission denied', path)\n return _orig(path,*args,**kwargs)\nos.open=guarded\n`);
  await writeFile(join(commands, "curl"), '#!/bin/sh\ntouch "$NETWORK_CALLED"\nexit 1\n', {mode:0o755});
  const env = {...process.env, HOME:join(root,"home"), XDG_DATA_HOME:join(root,"xdg"), GLADE_INSTALL_DIR:join(root,"bin"), PATH:`${commands}:${process.env.PATH}`, PYTHONPATH:commands, NETWORK_CALLED:join(root,"network")};
  delete env.GLADE_HOME; delete env.GLADE_ROOT;
  const result = spawnSync("sh", [join(repo,script)], {env,encoding:"utf8",timeout:10000});
  assert.notEqual(result.status, 0);
  assert.match(result.stderr, /could not acquire toolchain install lock/);
  assert.equal(existsSync(env.NETWORK_CALLED), false, "download began without the install lock");
  assert.equal(existsSync(join(root,"bin","glade")), false, "binary was replaced without the install lock");
});
