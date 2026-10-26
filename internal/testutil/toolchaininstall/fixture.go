package toolchaininstall

import (
	"path/filepath"
	"testing"
)

func sourceFixture(t *testing.T, src string) []string {
	t.Helper()
	files := []string{
		"go.mod",
		"third_party/lwc/compile.mjs",
		"third_party/lwc/node_modules/@lwc/compiler/index.js",
		"third_party/lwc/node_modules/@lwc/engine-dom/dist/index.js",
		"lwcruntime/src/experience/cmsDeliveryApi.mjs",
		"lwcruntime/src/lightning/button.mjs",
		"lwcruntime/src/shell/app.mjs",
		"lwcruntime/src/shims/wire-adapter.mjs",
		"lwcruntime/src/shims/lds-cache.mjs",
		"lwcruntime/src/slds/slds-loader.mjs",
		"contrib/vscode-glade/dist/vscode-glade-0.0.1.vsix",
	}
	for _, rel := range files {
		writeFile(t, filepath.Join(src, filepath.FromSlash(rel)), rel, 0o644)
	}
	writeFile(t, filepath.Join(src, "third_party/lwc/compile.mjs"), `import fs from 'node:fs';
import path from 'node:path';
const cfg = JSON.parse(fs.readFileSync(0, 'utf8'));
const share = path.join(process.env.XDG_DATA_HOME, 'glade');
fs.readFileSync(path.join(share, 'editor/vscode-glade.vsix'));
if (process.argv.includes('--salesforce-import-references')) {
 process.stdout.write('[]');
} else {
 const emit = out => {
  const file = path.join(out, 'c/ready/ready.js');
  fs.mkdirSync(path.dirname(file), {recursive: true});
  fs.writeFileSync(file, "export default 'ready';");
  return {'c/ready': {moduleKey:'c/ready', tag:'c-ready', file}};
 };
 const bundles = Object.fromEntries(Object.entries(cfg.bundleOutDirs || {}).map(([key,out]) => [key,{modules:emit(out)}]));
 process.stdout.write(JSON.stringify({modules:emit(cfg.outDir), bundles}));
}
`, 0o644)
	return files
}
