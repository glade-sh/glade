# Extend Runtime Support

Start with a small failing proof. Write the failing fixture or product test first.
Then patch the runtime.

## Loop

1. Name the unsupported Apex surface.
2. Add the smallest fixture, parser case, VM test, or black-box test that shows
   the gap.
3. Patch product code in Glade.
4. Run the narrow proof.
5. Regenerate support docs only when generated inputs changed.
6. Run the product proof before release.

## Common proof commands

```bash
CGO_ENABLED=1 go test ./internal/vm ./internal/apextest
CGO_ENABLED=1 go test ./internal/sema ./internal/apexast
CGO_ENABLED=1 go test ./internal/gladecli
npm test --prefix site
```

Use Salesforce behavior, public grammars, owned fixtures, or black-box tests as
the source of truth. Do not infer field behavior from field names. Do not add
project-specific exceptions to product code.

## Generated docs

Generated compatibility reports come from the maintainer toolkit. The product
docs site renders selected outputs. Product pages should describe what users can
do. Maintainer pages can show how the rows were cut.
