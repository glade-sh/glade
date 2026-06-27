# Help Project

`setup.mjs` writes a disposable SFDX project to `.glade/macrodata-apex`.

Use it for screenshots only. Do not edit the generated project by hand.

```bash
npm --prefix site run help:project
```

The project contains:

- `sfdx-project.json`
- `RefinementService.cls`
- `RefinementServiceTest.cls`
- `anonymous.apex`
- `seed.json`
- `data/insertOrder.json`
- `data/accounts.json`
- `data/contacts.json`
- `reports/anonymous-output.txt`
