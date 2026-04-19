from __future__ import annotations

import json
from dataclasses import dataclass
from typing import Any

HEADER = "lattice 1."


class LatticeError(ValueError):
    """Raised when a Launchpad Lattice document is invalid."""


@dataclass(frozen=True)
class _LogicalLine:
    indent: int
    text: str
    number: int


class _ContainerKind:
    RECORD = "record"
    LIST = "list"


def parse_document(content: str) -> Any:
    lines = _logical_lines(content)
    if not lines:
        return {}

    index = 0
    if lines[0].text in {"lattice 1.", "ldf 1."}:
        index += 1

    while index < len(lines):
        line = lines[index]
        if line.indent != 0:
            raise LatticeError(
                f"expected a top-level Launchpad Lattice statement at line {line.number}"
            )
        if line.text == "data:":
            value, _ = _parse_container(lines, index + 1, 2, _ContainerKind.RECORD)
            return value
        kind = _block_kind_after_prefix(line.text, "data is ")
        if kind is not None:
            value, _ = _parse_container(lines, index + 1, 2, kind)
            return value
        if line.text.startswith("data is ") and line.text.endswith("."):
            expr = line.text[len("data is ") : -1].strip()
            return _parse_scalar(expr)
        index += 1

    raise LatticeError("Launchpad Lattice document must define a `data` payload")


def loads(content: str) -> Any:
    return parse_document(content)


def loadb(data: bytes) -> Any:
    return parse_document(data.decode("utf-8"))


def dumps(value: Any) -> str:
    return to_string_pretty(value)


def dumpb(value: Any) -> bytes:
    return to_bytes(value)


def to_string_pretty(value: Any) -> str:
    out: list[str] = [HEADER, "\n", "\n"]
    if isinstance(value, dict):
        out.append("data:\n")
        _render_record_entries(value, 2, out)
    elif isinstance(value, list):
        out.append("data is list:\n")
        _render_list_items(value, 2, out)
    else:
        out.append(f"data is {_render_scalar(value)}.\n")
    return "".join(out)


def to_bytes(value: Any) -> bytes:
    return to_string_pretty(value).encode("utf-8")


def _logical_lines(content: str) -> list[_LogicalLine]:
    lines: list[_LogicalLine] = []
    for offset, raw_line in enumerate(content.splitlines()):
        if "\t" in raw_line:
            raise LatticeError(
                f"tabs are not allowed in Launchpad Lattice line {offset + 1}"
            )
        stripped = _strip_comment(raw_line)
        if not stripped.strip():
            continue
        indent = len(stripped) - len(stripped.lstrip(" "))
        lines.append(
            _LogicalLine(indent=indent, text=stripped[indent:].rstrip(), number=offset + 1)
        )
    return lines


def _strip_comment(line: str) -> str:
    out: list[str] = []
    in_string = False
    escaped = False
    for ch in line:
        if escaped:
            out.append(ch)
            escaped = False
            continue
        if ch == "\\" and in_string:
            out.append(ch)
            escaped = True
            continue
        if ch == '"':
            in_string = not in_string
            out.append(ch)
            continue
        if ch == "#" and not in_string:
            break
        out.append(ch)
    return "".join(out)


def _parse_container(
    lines: list[_LogicalLine], index: int, indent: int, kind: str
) -> tuple[Any, int]:
    if kind == _ContainerKind.RECORD:
        result: dict[str, Any] = {}
        while index < len(lines):
            line = lines[index]
            if line.indent < indent:
                break
            if line.indent != indent:
                raise LatticeError(
                    f"unexpected indentation at line {line.number}: expected {indent} spaces"
                )
            key, value, index = _parse_record_entry(lines, index, indent)
            if key in result:
                raise LatticeError(f"duplicate key `{key}` at line {line.number}")
            result[key] = value
        return result, index

    result_list: list[Any] = []
    while index < len(lines):
        line = lines[index]
        if line.indent < indent:
            break
        if line.indent != indent:
            raise LatticeError(
                f"unexpected indentation at line {line.number}: expected {indent} spaces"
            )
        value, index = _parse_list_item(lines, index, indent)
        result_list.append(value)
    return result_list, index


def _parse_record_entry(
    lines: list[_LogicalLine], index: int, indent: int
) -> tuple[str, Any, int]:
    line = lines[index]
    if line.text.startswith("entry "):
        return _parse_named_value_entry(line.text[len("entry ") :], lines, index, indent)
    return _parse_named_value_entry(line.text, lines, index, indent)


def _parse_named_value_entry(
    text: str, lines: list[_LogicalLine], index: int, indent: int
) -> tuple[str, Any, int]:
    line = lines[index]
    if " is " in text:
        name, expr = text.split(" is ", 1)
        key = _parse_key(name.strip())
        trimmed = expr.rstrip(":").strip()
        kind = _block_kind(trimmed)
        if kind is not None:
            if not text.rstrip().endswith(":"):
                raise LatticeError(f"expected block entry at line {line.number}")
            value, next_index = _parse_container(lines, index + 1, indent + 2, kind)
            return key, value, next_index
        if not expr.strip().endswith("."):
            raise LatticeError(f"expected `.` at line {line.number}")
        return key, _parse_scalar(expr.strip()[:-1].strip()), index + 1
    if text.endswith(":"):
        key = _parse_key(text[:-1].strip())
        value, next_index = _parse_container(lines, index + 1, indent + 2, _ContainerKind.RECORD)
        return key, value, next_index
    raise LatticeError(f"invalid record entry at line {line.number}")


def _parse_list_item(
    lines: list[_LogicalLine], index: int, indent: int
) -> tuple[Any, int]:
    line = lines[index]
    if not line.text.startswith("item is "):
        raise LatticeError(f"list items must start with `item is` at line {line.number}")
    rest = line.text[len("item is ") :]
    trimmed = rest.rstrip(":").strip()
    kind = _block_kind(trimmed)
    if kind is not None:
        if not line.text.endswith(":"):
            raise LatticeError(f"expected block list item at line {line.number}")
        return _parse_container(lines, index + 1, indent + 2, kind)
    if not rest.strip().endswith("."):
        raise LatticeError(f"expected `.` at line {line.number}")
    return _parse_scalar(rest.strip()[:-1].strip()), index + 1


def _parse_key(raw: str) -> str:
    if raw.startswith('"'):
        value = _parse_string(raw)
        if not isinstance(value, str):
            raise LatticeError("key must be a string")
        return value
    if not raw:
        raise LatticeError("empty key is not allowed")
    return raw


def _block_kind(expr: str) -> str | None:
    if expr in {"list", "set"}:
        return _ContainerKind.LIST
    if expr == "record" or expr.startswith("map of "):
        return _ContainerKind.RECORD
    if expr and all(ch.isascii() and (ch.isalnum() or ch == "_") for ch in expr):
        return _ContainerKind.RECORD
    return None


def _block_kind_after_prefix(text: str, prefix: str) -> str | None:
    if not text.startswith(prefix) or not text.endswith(":"):
        return None
    return _block_kind(text[len(prefix) : -1].strip())


def _parse_scalar(expr: str) -> Any:
    if expr == "none":
        return None
    if expr == "true":
        return True
    if expr == "false":
        return False
    if expr.startswith('"'):
        return _parse_string(expr)
    tagged = _parse_tagged_string(expr)
    if tagged is not None:
        return tagged
    try:
        return int(expr)
    except ValueError:
        pass
    try:
        return float(expr)
    except ValueError:
        pass
    return expr


def _parse_tagged_string(expr: str) -> str | None:
    parts = expr.split(" ", 1)
    if len(parts) != 2:
        return None
    rest = parts[1].strip()
    if rest.startswith('"'):
        value = _parse_string(rest)
        return value if isinstance(value, str) else None
    tail_parts = rest.split(" ", 1)
    if len(tail_parts) == 2 and tail_parts[1].strip().startswith('"'):
        value = _parse_string(tail_parts[1].strip())
        return value if isinstance(value, str) else None
    return None


def _parse_string(expr: str) -> Any:
    try:
        return json.loads(expr)
    except json.JSONDecodeError as exc:
        raise LatticeError(str(exc)) from exc


def _render_record_entries(value: dict[str, Any], indent: int, out: list[str]) -> None:
    for key in sorted(value):
        _render_named_value(key, value[key], indent, out)


def _render_list_items(values: list[Any], indent: int, out: list[str]) -> None:
    for value in values:
        _render_item_value(value, indent, out)


def _render_named_value(name: str, value: Any, indent: int, out: list[str]) -> None:
    prefix = (
        f'{" " * indent}{name} is '
        if _is_identifier(name)
        else f'{" " * indent}entry {json.dumps(name)} is '
    )
    if isinstance(value, dict):
        out.append(f"{prefix}record:\n")
        _render_record_entries(value, indent + 2, out)
        return
    if isinstance(value, list):
        out.append(f"{prefix}list:\n")
        _render_list_items(value, indent + 2, out)
        return
    out.append(f"{prefix}{_render_scalar(value)}.\n")


def _render_item_value(value: Any, indent: int, out: list[str]) -> None:
    prefix = f'{" " * indent}item is '
    if isinstance(value, dict):
        out.append(f"{prefix}record:\n")
        _render_record_entries(value, indent + 2, out)
        return
    if isinstance(value, list):
        out.append(f"{prefix}list:\n")
        _render_list_items(value, indent + 2, out)
        return
    out.append(f"{prefix}{_render_scalar(value)}.\n")


def _render_scalar(value: Any) -> str:
    if value is None:
        return "none"
    if isinstance(value, bool):
        return "true" if value else "false"
    if isinstance(value, int) and not isinstance(value, bool):
        return str(value)
    if isinstance(value, float):
        return json.dumps(value)
    if isinstance(value, str):
        return json.dumps(value)
    raise LatticeError(f"cannot render non-scalar Launchpad Lattice value `{value}`")


def _is_identifier(value: str) -> bool:
    if not value or not value[0].isascii() or not value[0].isalpha():
        return False
    return all(ch.isascii() and (ch.isalnum() or ch == "_") for ch in value[1:])
