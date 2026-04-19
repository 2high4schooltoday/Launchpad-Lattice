# Launchpad Lattice for JavaScript

Pure JavaScript parser and renderer for Launchpad Lattice documents.

## API

```js
const { parse, stringify } = require("launchpad-lattice");

const value = parse(`
lattice 1.

data:
  appId is "hello".
  enabled is true.
`);

const rendered = stringify(value);
```
