# Launchpad Lattice

Launchpad Lattice is a reviewable, deterministic data format designed for
Launchpad-style operational data.

It is intended for:

- layered defaults
- structured config payloads
- manifests
- receipts
- local state snapshots

The canonical file extension is `.l`.

## Libraries

This repository now includes native implementations for:

- Rust crate: `launchpad-lattice` / `launchpad_lattice`
- Python package: `python/launchpad_lattice`
- JavaScript package: `javascript`
- Go module: `go`

## Example

```l
lattice 1.

data:
  appId is "hello".
  enabled is true.
  retries is 3.
  architectures is list:
    item is "linux-x86_64".
    item is "linux-arm64".
```

## Rust Usage

```rust
use launchpad_lattice::{from_str, to_string_pretty};

#[derive(serde::Serialize, serde::Deserialize)]
struct Defaults {
    app_id: String,
    enabled: bool,
}

let parsed: Defaults = from_str(
    "lattice 1.\n\ndata:\n  app_id is \"hello\".\n  enabled is true.\n",
)?;

let rendered = to_string_pretty(&parsed)?;
```

## Python Usage

```python
from launchpad_lattice import dumps, loads

parsed = loads(
    "lattice 1.\n\ndata:\n  appId is \"hello\".\n  enabled is true.\n"
)

rendered = dumps(parsed)
```

## JavaScript Usage

```js
const { parse, stringify } = require("./javascript");

const parsed = parse(`
lattice 1.

data:
  appId is "hello".
  enabled is true.
`);

const rendered = stringify(parsed);
```

## Go Usage

```go
package main

import lattice "github.com/2high4schooltoday/Launchpad-Lattice/go"

func main() {
	parsed, _ := lattice.ParseDocument("lattice 1.\n\ndata:\n  appId is \"hello\".\n")
	_, _ = lattice.Marshal(parsed)
}
```

## Test Coverage

Each implementation includes the same baseline behavior checks:

- parse structured records, lists, and tagged strings
- ignore non-`data` sections when `data` is present
- round-trip render and parse
- reject duplicate keys

## Specification

The format specification lives at [docs/spec/launchpad-lattice.md](docs/spec/launchpad-lattice.md).
