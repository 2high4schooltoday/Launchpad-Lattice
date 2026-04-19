# Launchpad Lattice for Python

Pure Python parser and renderer for Launchpad Lattice documents.

## API

```python
from launchpad_lattice import dumps, loads

value = loads("""
lattice 1.

data:
  appId is "hello".
  enabled is true.
""")

rendered = dumps(value)
```
