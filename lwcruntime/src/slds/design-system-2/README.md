# Design System 2

**Design System 2 (SLDS 2)** is the next-generation design system for Salesforce products. It provides component-based styling, themeable design tokens, and abstract component definitions (UIF) for building consistent, accessible, and scalable user interfaces.

## Installation

```bash
yarn add @salesforce-ux/design-system-2
```

## Dist Structure

The published package is organized by asset type, then by consumption intent.

| Path              | Contents                                      | Audience                  |
| ----------------- | --------------------------------------------- | ------------------------- |
| `css/bundled/`    | Single CSS file with structure + theme tokens | Monolith consumers        |
| `css/modular/`    | Separate base + theme CSS files               | Theme-swapping consumers  |
| `components/`     | Per-component static assets (source CSS, UIF) | Component-level consumers |
| `uif/slds/`       | SLDS system component definitions             | Most consuming teams      |
| `uif/foundation/` | Abstract component definitions                | Component automation      |

## CSS

### Bundled (single file)

Each bundled file is self-contained: component structure and theme tokens in one stylesheet. No additional files needed.

```html
<link rel="stylesheet" href="@salesforce-ux/design-system-2/dist/css/bundled/slds2.cosmos.css" />
```

Available bundled files:

| File                              | Description                             |
| --------------------------------- | --------------------------------------- |
| `slds2.cosmos.css`                | Structure + Cosmos theme                |
| `slds2.lightning-blue.css`        | Structure + Lightning Blue theme        |
| `slds2.scoped.cosmos.css`         | Scoped structure + Cosmos theme         |
| `slds2.scoped.lightning-blue.css` | Scoped structure + Lightning Blue theme |

### Modular (theme-swapping)

Load the base structure CSS once, then swap the theme stylesheet to change themes at runtime.

```html
<!-- Always load the base structure -->
<link rel="stylesheet" href="@salesforce-ux/design-system-2/dist/css/modular/slds2.base.css" />

<!-- Load the active theme (swap this href to switch themes) -->
<link
  rel="stylesheet"
  href="@salesforce-ux/design-system-2/dist/css/modular/slds2.theme.cosmos.css"
  id="theme-css"
/>
```

To switch themes at runtime, update the theme stylesheet's `href`:

```javascript
document.getElementById('theme-css').href =
  '@salesforce-ux/design-system-2/dist/css/modular/slds2.theme.lightning-blue.css';
```

Available modular files:

| File                             | Description                                                         |
| -------------------------------- | ------------------------------------------------------------------- |
| `slds2.base.css`                 | Component structure, utilities, hook declarations (no theme tokens) |
| `slds2.scoped.base.css`          | Scoped variant of the base structure                                |
| `slds2.theme.cosmos.css`         | Cosmos theme token declarations                                     |
| `slds2.theme.lightning-blue.css` | Lightning Blue theme token declarations                             |

Loading `slds2.base.css` + `slds2.theme.cosmos.css` together is functionally equivalent to loading `slds2.cosmos.css` from the bundled directory.

### Scoped Variants

Scoped CSS files wrap all component selectors under a `.slds-scope` class. Theme token files are shared between scoped and unscoped builds because token declarations target `:where(html)`, which is unaffected by selector scoping.

To use scoped styles with theme-swapping, pair `slds2.scoped.base.css` with any `slds2.theme.*.css` file.

## Components (per-component static assets)

The `dist/components/` directory provides individual component assets, allowing consumers to import only what they need.

Each component directory follows this structure:

```text
components/{componentName}/
  {componentName}.css              # Structure CSS (selectors, layout)
  {componentName}.deprecated.css   # Deprecated selectors (if applicable)
  {componentName}.behavior.ts      # Behavior definition (if applicable)
  themes/
    base.css                       # Shared theme-layer defaults
    cosmos.css                     # Cosmos theme values (if migrated)
    lightning-blue.css             # Lightning Blue theme values (if migrated)
```

### Structure CSS

The `{componentName}.css` file contains the component's structural styles: selectors, layout rules, and hook consumption via `var()`. It does not include theme token values.

```javascript
import buttonCss from '@salesforce-ux/design-system-2/dist/components/button/button.css';
```

### Theme files

The `themes/` subdirectory contains theme-layer CSS for the component.

- `base.css` is present for every component and contains shared structural defaults.
- Per-theme files (e.g. `cosmos.css`) contain theme-specific hook values and are present for components that have been migrated to the theme layer.

```javascript
import buttonThemeBase from '@salesforce-ux/design-system-2/dist/components/button/themes/base.css';
import buttonThemeCosmos from '@salesforce-ux/design-system-2/dist/components/button/themes/cosmos.css';
```

## UIF (User Interface Format)

> **Note:** UIF is prototypal, pre-versioned, and subject to change at any time. Do not take a dependency on the schema or file structure without coordinating with the Design Systems Engineering team.

Two types of UIF definitions are provided:

### SLDS system definitions (most teams)

Resolved definitions that include SLDS CSS class names, variant options, and modifier mappings. Use these when building components that render SLDS markup.

```javascript
import buttonDef from '@salesforce-ux/design-system-2/dist/uif/slds/button.uif.json';
```

### Foundation definitions (cross-surface baselines)

Abstract definitions that describe component structure without SLDS-specific CSS. These serve as cross-surface baselines for component automation and validation.

```javascript
import buttonDef from '@salesforce-ux/design-system-2/dist/uif/foundation/button.uif.json';
```

## Available Themes

| Theme          | Token prefix     | Description                        |
| -------------- | ---------------- | ---------------------------------- |
| Cosmos         | `cosmos`         | Default SLDS 2 theme               |
| Lightning Blue | `lightning-blue` | Classic Salesforce Lightning theme |

## CSS Cascade Layers

SLDS 2 CSS uses `@layer` to manage cascade priority. The layer order is declared at the top of every base and bundled CSS file:

```css
@layer deprecated, defaults, shared, theme, component;
```

This ensures tokens and component styles cascade correctly regardless of stylesheet load order. Theme files do not redeclare the layer order; they participate in the layers established by the base file.

## License

See [LICENSE.txt](LICENSE.txt) in this package.
