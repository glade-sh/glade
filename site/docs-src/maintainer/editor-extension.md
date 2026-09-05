---
pageType: contributor
canonicalTask: /maintainer/editor-extension
---

# Develop and package the VS Code extension

<div class="docs-intro">
  <p class="docs-intro-eyebrow">Maintainer</p>
  <p>Install dependencies, package the checked extension source, and load the resulting VSIX through the product CLI.</p>
</div>

Run from the product repository checkout:

```bash
npm ci --prefix contrib/vscode-glade
npm test --prefix contrib/vscode-glade
npm --prefix contrib/vscode-glade run package
```

Use the VSIX path printed by the package command with
`glade editor install vscode --vsix <packaged-vsix-path> --force`. This makes
the candidate explicit instead of accidentally reinstalling another bundled
release.

The packaged release installs the VSIX at
`share/glade/editor/vscode-glade.vsix`. Keep end-user setup and workflows in
[Use Glade in VS Code](/guide/editor); keep protocol behavior in the [LSP
reference](/reference/lsp) and [DAP reference](/reference/dap).

Before publishing, run the extension checks defined in
`contrib/vscode-glade/package.json` and the product release check. Do not check a
generated VSIX into the repository.
