# Launchpad Lattice for Go

Pure Go parser and renderer for Launchpad Lattice documents.

## API

```go
package main

import lattice "github.com/2high4schooltoday/Launchpad-Lattice/go"

func main() {
	value, _ := lattice.ParseDocument(`
lattice 1.

data:
  appId is "hello".
  enabled is true.
`)

	_, _ = lattice.Marshal(value)
}
```
