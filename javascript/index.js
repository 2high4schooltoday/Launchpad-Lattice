"use strict";

const HEADER = "lattice 1.";

class LatticeError extends Error {
  constructor(message) {
    super(message);
    this.name = "LatticeError";
  }
}

const ContainerKind = {
  RECORD: "record",
  LIST: "list",
};

function parseDocument(content) {
  const lines = logicalLines(content);
  if (lines.length === 0) {
    return {};
  }

  let index = 0;
  if (lines[0].text === "lattice 1." || lines[0].text === "ldf 1.") {
    index += 1;
  }

  while (index < lines.length) {
    const line = lines[index];
    if (line.indent !== 0) {
      throw new LatticeError(
        `expected a top-level Launchpad Lattice statement at line ${line.number}`,
      );
    }
    if (line.text === "data:") {
      return parseContainer(lines, index + 1, 2, ContainerKind.RECORD)[0];
    }
    const kind = blockKindAfterPrefix(line.text, "data is ");
    if (kind !== null) {
      return parseContainer(lines, index + 1, 2, kind)[0];
    }
    if (line.text.startsWith("data is ") && line.text.endsWith(".")) {
      return parseScalar(line.text.slice("data is ".length, -1).trim());
    }
    index += 1;
  }

  throw new LatticeError("Launchpad Lattice document must define a `data` payload");
}

function parse(content) {
  return parseDocument(content);
}

function parseBuffer(buffer) {
  return parseDocument(Buffer.from(buffer).toString("utf8"));
}

function stringify(value) {
  const out = [HEADER, "\n", "\n"];
  if (isRecord(value)) {
    out.push("data:\n");
    renderRecordEntries(value, 2, out);
  } else if (Array.isArray(value)) {
    out.push("data is list:\n");
    renderListItems(value, 2, out);
  } else {
    out.push(`data is ${renderScalar(value)}.\n`);
  }
  return out.join("");
}

function toBuffer(value) {
  return Buffer.from(stringify(value), "utf8");
}

function logicalLines(content) {
  const rawLines = content.split(/\r?\n/u);
  const lines = [];
  for (let i = 0; i < rawLines.length; i += 1) {
    const rawLine = rawLines[i];
    if (rawLine.includes("\t")) {
      throw new LatticeError(
        `tabs are not allowed in Launchpad Lattice line ${i + 1}`,
      );
    }
    const stripped = stripComment(rawLine);
    if (stripped.trim() === "") {
      continue;
    }
    const indent = stripped.length - stripped.replace(/^ */u, "").length;
    lines.push({
      indent,
      text: stripped.slice(indent).replace(/[ ]+$/u, ""),
      number: i + 1,
    });
  }
  return lines;
}

function stripComment(line) {
  let out = "";
  let inString = false;
  let escaped = false;
  for (const ch of line) {
    if (escaped) {
      out += ch;
      escaped = false;
      continue;
    }
    if (ch === "\\" && inString) {
      out += ch;
      escaped = true;
      continue;
    }
    if (ch === '"') {
      inString = !inString;
      out += ch;
      continue;
    }
    if (ch === "#" && !inString) {
      break;
    }
    out += ch;
  }
  return out;
}

function parseContainer(lines, index, indent, kind) {
  if (kind === ContainerKind.RECORD) {
    const record = {};
    while (index < lines.length) {
      const line = lines[index];
      if (line.indent < indent) {
        break;
      }
      if (line.indent !== indent) {
        throw new LatticeError(
          `unexpected indentation at line ${line.number}: expected ${indent} spaces`,
        );
      }
      const [key, value, nextIndex] = parseRecordEntry(lines, index, indent);
      if (Object.prototype.hasOwnProperty.call(record, key)) {
        throw new LatticeError(`duplicate key \`${key}\` at line ${line.number}`);
      }
      record[key] = value;
      index = nextIndex;
    }
    return [record, index];
  }

  const values = [];
  while (index < lines.length) {
    const line = lines[index];
    if (line.indent < indent) {
      break;
    }
    if (line.indent !== indent) {
      throw new LatticeError(
        `unexpected indentation at line ${line.number}: expected ${indent} spaces`,
      );
    }
    const [value, nextIndex] = parseListItem(lines, index, indent);
    values.push(value);
    index = nextIndex;
  }
  return [values, index];
}

function parseRecordEntry(lines, index, indent) {
  const line = lines[index];
  if (line.text.startsWith("entry ")) {
    return parseNamedValueEntry(line.text.slice("entry ".length), lines, index, indent);
  }
  return parseNamedValueEntry(line.text, lines, index, indent);
}

function parseNamedValueEntry(text, lines, index, indent) {
  const line = lines[index];
  const splitAt = text.indexOf(" is ");
  if (splitAt !== -1) {
    const key = parseKey(text.slice(0, splitAt).trim());
    const expr = text.slice(splitAt + 4);
    const kind = blockKind(expr.replace(/:$/u, "").trim());
    if (kind !== null) {
      if (!text.trimEnd().endsWith(":")) {
        throw new LatticeError(`expected block entry at line ${line.number}`);
      }
      const [value, nextIndex] = parseContainer(lines, index + 1, indent + 2, kind);
      return [key, value, nextIndex];
    }
    if (!expr.trim().endsWith(".")) {
      throw new LatticeError(`expected \`.\` at line ${line.number}`);
    }
    return [key, parseScalar(expr.trim().slice(0, -1).trim()), index + 1];
  }
  if (text.endsWith(":")) {
    const key = parseKey(text.slice(0, -1).trim());
    const [value, nextIndex] = parseContainer(
      lines,
      index + 1,
      indent + 2,
      ContainerKind.RECORD,
    );
    return [key, value, nextIndex];
  }
  throw new LatticeError(`invalid record entry at line ${line.number}`);
}

function parseListItem(lines, index, indent) {
  const line = lines[index];
  if (!line.text.startsWith("item is ")) {
    throw new LatticeError(`list items must start with \`item is\` at line ${line.number}`);
  }
  const rest = line.text.slice("item is ".length);
  const kind = blockKind(rest.replace(/:$/u, "").trim());
  if (kind !== null) {
    if (!line.text.endsWith(":")) {
      throw new LatticeError(`expected block list item at line ${line.number}`);
    }
    return parseContainer(lines, index + 1, indent + 2, kind);
  }
  if (!rest.trim().endsWith(".")) {
    throw new LatticeError(`expected \`.\` at line ${line.number}`);
  }
  return [parseScalar(rest.trim().slice(0, -1).trim()), index + 1];
}

function parseKey(raw) {
  if (raw.startsWith('"')) {
    const value = parseString(raw);
    if (typeof value !== "string") {
      throw new LatticeError("key must be a string");
    }
    return value;
  }
  if (raw === "") {
    throw new LatticeError("empty key is not allowed");
  }
  return raw;
}

function blockKind(expr) {
  if (expr === "list" || expr === "set") {
    return ContainerKind.LIST;
  }
  if (expr === "record" || expr.startsWith("map of ")) {
    return ContainerKind.RECORD;
  }
  if (/^[A-Za-z0-9_]+$/u.test(expr) && expr !== "") {
    return ContainerKind.RECORD;
  }
  return null;
}

function blockKindAfterPrefix(text, prefix) {
  if (!text.startsWith(prefix) || !text.endsWith(":")) {
    return null;
  }
  return blockKind(text.slice(prefix.length, -1).trim());
}

function parseScalar(expr) {
  if (expr === "none") {
    return null;
  }
  if (expr === "true") {
    return true;
  }
  if (expr === "false") {
    return false;
  }
  if (expr.startsWith('"')) {
    return parseString(expr);
  }
  const tagged = parseTaggedString(expr);
  if (tagged !== null) {
    return tagged;
  }
  if (/^-?\d+$/u.test(expr)) {
    const parsed = Number.parseInt(expr, 10);
    if (Number.isSafeInteger(parsed)) {
      return parsed;
    }
  }
  if (/^[+]?\d+$/u.test(expr)) {
    const parsed = Number.parseInt(expr, 10);
    if (Number.isSafeInteger(parsed)) {
      return parsed;
    }
  }
  if (/^-?(?:\d+\.\d+|\d+[eE][+-]?\d+|\d+\.\d+[eE][+-]?\d+)$/u.test(expr)) {
    return Number.parseFloat(expr);
  }
  return expr;
}

function parseTaggedString(expr) {
  const firstSpace = expr.indexOf(" ");
  if (firstSpace === -1) {
    return null;
  }
  const rest = expr.slice(firstSpace + 1).trim();
  if (rest.startsWith('"')) {
    return parseString(rest);
  }
  const secondSpace = rest.indexOf(" ");
  if (secondSpace !== -1) {
    const tail = rest.slice(secondSpace + 1).trim();
    if (tail.startsWith('"')) {
      return parseString(tail);
    }
  }
  return null;
}

function parseString(expr) {
  try {
    return JSON.parse(expr);
  } catch (error) {
    throw new LatticeError(error.message);
  }
}

function renderRecordEntries(record, indent, out) {
  for (const key of Object.keys(record).sort()) {
    renderNamedValue(key, record[key], indent, out);
  }
}

function renderListItems(values, indent, out) {
  for (const value of values) {
    renderItemValue(value, indent, out);
  }
}

function renderNamedValue(name, value, indent, out) {
  const prefix = isIdentifier(name)
    ? `${" ".repeat(indent)}${name} is `
    : `${" ".repeat(indent)}entry ${JSON.stringify(name)} is `;
  if (isRecord(value)) {
    out.push(`${prefix}record:\n`);
    renderRecordEntries(value, indent + 2, out);
    return;
  }
  if (Array.isArray(value)) {
    out.push(`${prefix}list:\n`);
    renderListItems(value, indent + 2, out);
    return;
  }
  out.push(`${prefix}${renderScalar(value)}.\n`);
}

function renderItemValue(value, indent, out) {
  const prefix = `${" ".repeat(indent)}item is `;
  if (isRecord(value)) {
    out.push(`${prefix}record:\n`);
    renderRecordEntries(value, indent + 2, out);
    return;
  }
  if (Array.isArray(value)) {
    out.push(`${prefix}list:\n`);
    renderListItems(value, indent + 2, out);
    return;
  }
  out.push(`${prefix}${renderScalar(value)}.\n`);
}

function renderScalar(value) {
  if (value === null) {
    return "none";
  }
  if (typeof value === "boolean" || typeof value === "number") {
    return String(value);
  }
  if (typeof value === "string") {
    return JSON.stringify(value);
  }
  throw new LatticeError(`cannot render non-scalar Launchpad Lattice value \`${value}\``);
}

function isIdentifier(value) {
  return /^[A-Za-z][A-Za-z0-9_]*$/u.test(value);
}

function isRecord(value) {
  return value !== null && typeof value === "object" && !Array.isArray(value);
}

module.exports = {
  LatticeError,
  parse,
  parseBuffer,
  parseDocument,
  stringify,
  toBuffer,
};
