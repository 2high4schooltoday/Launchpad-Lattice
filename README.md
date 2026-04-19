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

## Crate

This repository publishes the Rust crate:

- package: `launchpad-lattice`
- library: `launchpad_lattice`

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

## Specification

The format specification lives at [docs/spec/launchpad-lattice.md](docs/spec/launchpad-lattice.md).
