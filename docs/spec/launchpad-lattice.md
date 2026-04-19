# Launchpad Lattice Specification

Launchpad Lattice is a strict, Launchpad-native data interchange format.

The goal is not to replace every external serialization format on earth. The
goal is to give Launchpad a default data shape that is explicit, greppable,
reviewable, deterministic, and easy to reason about in terminals and code
reviews.

## File Extension

Launchpad Lattice files use the `.l` extension.

## Header

Canonical documents begin with:

```l
lattice 1.
```

`ldf 1.` may be accepted as a compatibility alias by some parsers, but
documentation and examples should use `lattice 1.`.

## Payload Shape

The primary payload lives in a `data:` block:

```l
lattice 1.

data:
  appId is "hello".
  enabled is true.
  retries is 3.
```

Parsers may also accept `data is ...` for scalar or typed payload entrypoints.

## Lexical Rules

- Encoding: UTF-8
- Comments: `#` to end of line
- Indentation: spaces only
- Simple statements end with `.`
- Block headers end with `:`

## Supported Value Shapes

Current Launchpad Lattice v1 support covers:

- strings
- booleans
- `none`
- integers
- floating-point values
- records
- lists
- tagged string literals such as `path "/opt/app"` and `uri "file:bundle.lext"`

Tagged literals currently deserialize as strings unless a higher-level schema
layer applies stronger typing.

## Example Defaults

```l
lattice 1.

data:
  installDir is "/opt/app".
  configDir is "/etc/app".
  logDir is "/var/log/app".
```

## Design Constraints

- no parser network access
- no execution
- no shell interpolation
- duplicate keys are fatal
- comments do not affect the semantic payload
