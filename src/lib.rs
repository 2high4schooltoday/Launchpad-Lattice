use std::collections::BTreeMap;

use anyhow::{Result, anyhow, bail};
use serde::Serialize;
use serde::de::DeserializeOwned;
use serde_json::{Map, Number, Value};

const HEADER: &str = "lattice 1.";

#[derive(Debug, Clone)]
struct LogicalLine {
    indent: usize,
    text: String,
    number: usize,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
enum ContainerKind {
    Record,
    List,
}

pub fn from_str<T: DeserializeOwned>(content: &str) -> Result<T> {
    let value = parse_document(content)?;
    Ok(serde_json::from_value(value)?)
}

pub fn from_slice<T: DeserializeOwned>(bytes: &[u8]) -> Result<T> {
    from_str(std::str::from_utf8(bytes)?)
}

pub fn parse_document(content: &str) -> Result<Value> {
    let lines = logical_lines(content)?;
    if lines.is_empty() {
        return Ok(Value::Object(Map::new()));
    }

    let mut index = 0;
    if matches!(
        lines.first().map(|line| line.text.as_str()),
        Some("lattice 1." | "ldf 1.")
    ) {
        index += 1;
    }

    while index < lines.len() {
        let line = &lines[index];
        if line.indent != 0 {
            bail!(
                "expected a top-level Launchpad Lattice statement at line {}",
                line.number
            );
        }
        if line.text == "data:" {
            let (value, _) = parse_container(&lines, index + 1, 2, ContainerKind::Record)?;
            return Ok(value);
        }
        if let Some(kind) = block_kind_after_prefix(&line.text, "data is ") {
            let (value, _) = parse_container(&lines, index + 1, 2, kind)?;
            return Ok(value);
        }
        if line.text.starts_with("data is ") && line.text.ends_with('.') {
            let expr = line
                .text
                .strip_prefix("data is ")
                .and_then(|text| text.strip_suffix('.'))
                .ok_or_else(|| anyhow!("invalid data statement at line {}", line.number))?;
            return parse_scalar(expr.trim());
        }
        index += 1;
    }

    bail!("Launchpad Lattice document must define a `data` payload")
}

pub fn to_string_pretty<T: Serialize>(value: &T) -> Result<String> {
    let value = serde_json::to_value(value)?;
    let mut out = String::new();
    out.push_str(HEADER);
    out.push_str("\n\n");
    match value {
        Value::Object(map) => {
            out.push_str("data:\n");
            render_record_entries(&map, 2, &mut out);
        }
        Value::Array(values) => {
            out.push_str("data is list:\n");
            render_list_items(&values, 2, &mut out);
        }
        other => {
            out.push_str("data is ");
            out.push_str(&render_scalar(&other)?);
            out.push_str(".\n");
        }
    }
    Ok(out)
}

pub fn to_vec_pretty<T: Serialize>(value: &T) -> Result<Vec<u8>> {
    Ok(to_string_pretty(value)?.into_bytes())
}

fn logical_lines(content: &str) -> Result<Vec<LogicalLine>> {
    let mut lines = Vec::new();
    for (offset, raw_line) in content.lines().enumerate() {
        if raw_line.contains('\t') {
            bail!(
                "tabs are not allowed in Launchpad Lattice line {}",
                offset + 1
            );
        }
        let stripped = strip_comment(raw_line);
        if stripped.trim().is_empty() {
            continue;
        }
        let indent = stripped.chars().take_while(|ch| *ch == ' ').count();
        let text = stripped[indent..].trim_end().to_owned();
        lines.push(LogicalLine {
            indent,
            text,
            number: offset + 1,
        });
    }
    Ok(lines)
}

fn strip_comment(line: &str) -> String {
    let mut out = String::new();
    let mut chars = line.chars().peekable();
    let mut in_string = false;
    let mut escaped = false;
    while let Some(ch) = chars.next() {
        if escaped {
            out.push(ch);
            escaped = false;
            continue;
        }
        match ch {
            '\\' if in_string => {
                out.push(ch);
                escaped = true;
            }
            '"' => {
                in_string = !in_string;
                out.push(ch);
            }
            '#' if !in_string => break,
            _ => out.push(ch),
        }
    }
    out
}

fn parse_container(
    lines: &[LogicalLine],
    mut index: usize,
    indent: usize,
    kind: ContainerKind,
) -> Result<(Value, usize)> {
    match kind {
        ContainerKind::Record => {
            let mut map = Map::new();
            while index < lines.len() {
                let line = &lines[index];
                if line.indent < indent {
                    break;
                }
                if line.indent != indent {
                    bail!(
                        "unexpected indentation at line {}: expected {} spaces",
                        line.number,
                        indent
                    );
                }
                let (key, value, next_index) = parse_record_entry(lines, index, indent)?;
                if map.insert(key.clone(), value).is_some() {
                    bail!("duplicate key `{key}` at line {}", line.number);
                }
                index = next_index;
            }
            Ok((Value::Object(map), index))
        }
        ContainerKind::List => {
            let mut values = Vec::new();
            while index < lines.len() {
                let line = &lines[index];
                if line.indent < indent {
                    break;
                }
                if line.indent != indent {
                    bail!(
                        "unexpected indentation at line {}: expected {} spaces",
                        line.number,
                        indent
                    );
                }
                let (value, next_index) = parse_list_item(lines, index, indent)?;
                values.push(value);
                index = next_index;
            }
            Ok((Value::Array(values), index))
        }
    }
}

fn parse_record_entry(
    lines: &[LogicalLine],
    index: usize,
    indent: usize,
) -> Result<(String, Value, usize)> {
    let line = &lines[index];
    if let Some(rest) = line.text.strip_prefix("entry ") {
        return parse_named_value_entry(rest, lines, index, indent);
    }
    parse_named_value_entry(&line.text, lines, index, indent)
}

fn parse_named_value_entry(
    text: &str,
    lines: &[LogicalLine],
    index: usize,
    indent: usize,
) -> Result<(String, Value, usize)> {
    let line = &lines[index];
    if let Some((name, expr)) = text.split_once(" is ") {
        let key = parse_key(name.trim())?;
        if let Some(kind) = block_kind(expr.trim_end_matches(':').trim()) {
            if !text.trim_end().ends_with(':') {
                bail!("expected block entry at line {}", line.number);
            }
            let (value, next) = parse_container(lines, index + 1, indent + 2, kind)?;
            return Ok((key, value, next));
        }
        let expr = expr
            .trim()
            .strip_suffix('.')
            .ok_or_else(|| anyhow!("expected `.` at line {}", line.number))?;
        return Ok((key, parse_scalar(expr.trim())?, index + 1));
    }
    if let Some(name) = text.strip_suffix(':') {
        let key = parse_key(name.trim())?;
        let (value, next) = parse_container(lines, index + 1, indent + 2, ContainerKind::Record)?;
        return Ok((key, value, next));
    }
    bail!("invalid record entry at line {}", line.number)
}

fn parse_list_item(lines: &[LogicalLine], index: usize, indent: usize) -> Result<(Value, usize)> {
    let line = &lines[index];
    let Some(rest) = line.text.strip_prefix("item is ") else {
        bail!(
            "list items must start with `item is` at line {}",
            line.number
        );
    };
    if let Some(kind) = block_kind(rest.trim_end_matches(':').trim()) {
        if !line.text.ends_with(':') {
            bail!("expected block list item at line {}", line.number);
        }
        return parse_container(lines, index + 1, indent + 2, kind);
    }
    let expr = rest
        .trim()
        .strip_suffix('.')
        .ok_or_else(|| anyhow!("expected `.` at line {}", line.number))?;
    Ok((parse_scalar(expr.trim())?, index + 1))
}

fn parse_key(raw: &str) -> Result<String> {
    if raw.starts_with('"') {
        return parse_string(raw);
    }
    if raw.is_empty() {
        bail!("empty key is not allowed");
    }
    Ok(raw.to_owned())
}

fn block_kind(expr: &str) -> Option<ContainerKind> {
    if expr == "list" || expr == "set" {
        Some(ContainerKind::List)
    } else if expr == "record" || expr.starts_with("map of ") {
        Some(ContainerKind::Record)
    } else if expr
        .chars()
        .all(|ch| ch.is_ascii_alphanumeric() || ch == '_')
    {
        Some(ContainerKind::Record)
    } else {
        None
    }
}

fn block_kind_after_prefix(text: &str, prefix: &str) -> Option<ContainerKind> {
    let body = text.strip_prefix(prefix)?.strip_suffix(':')?.trim();
    block_kind(body)
}

fn parse_scalar(expr: &str) -> Result<Value> {
    if expr == "none" {
        return Ok(Value::Null);
    }
    if expr == "true" {
        return Ok(Value::Bool(true));
    }
    if expr == "false" {
        return Ok(Value::Bool(false));
    }
    if expr.starts_with('"') {
        return Ok(Value::String(parse_string(expr)?));
    }
    if let Some(tagged) = parse_tagged_string(expr)? {
        return Ok(Value::String(tagged));
    }
    if let Ok(value) = expr.parse::<i64>() {
        return Ok(Value::Number(Number::from(value)));
    }
    if let Ok(value) = expr.parse::<u64>() {
        return Ok(Value::Number(Number::from(value)));
    }
    if let Ok(value) = expr.parse::<f64>()
        && let Some(number) = Number::from_f64(value)
    {
        return Ok(Value::Number(number));
    }
    Ok(Value::String(expr.to_owned()))
}

fn parse_tagged_string(expr: &str) -> Result<Option<String>> {
    let Some((_, rest)) = expr.split_once(' ') else {
        return Ok(None);
    };
    let rest = rest.trim();
    if rest.starts_with('"') {
        return parse_string(rest).map(Some);
    }
    if let Some((_, tail)) = rest.split_once(' ') {
        let tail = tail.trim();
        if tail.starts_with('"') {
            return parse_string(tail).map(Some);
        }
    }
    Ok(None)
}

fn parse_string(expr: &str) -> Result<String> {
    Ok(serde_json::from_str(expr)?)
}

fn render_record_entries(map: &Map<String, Value>, indent: usize, out: &mut String) {
    let ordered: BTreeMap<_, _> = map.iter().collect();
    for (key, value) in ordered {
        render_named_value(key, value, indent, out);
    }
}

fn render_list_items(values: &[Value], indent: usize, out: &mut String) {
    for value in values {
        render_item_value(value, indent, out);
    }
}

fn render_named_value(name: &str, value: &Value, indent: usize, out: &mut String) {
    let prefix = if is_identifier(name) {
        format!("{}{} is ", " ".repeat(indent), name)
    } else {
        format!(
            "{}entry {} is ",
            " ".repeat(indent),
            serde_json::to_string(name).unwrap_or_else(|_| "\"\"".to_owned())
        )
    };
    match value {
        Value::Object(map) => {
            out.push_str(&prefix);
            out.push_str("record:\n");
            render_record_entries(map, indent + 2, out);
        }
        Value::Array(values) => {
            out.push_str(&prefix);
            out.push_str("list:\n");
            render_list_items(values, indent + 2, out);
        }
        _ => {
            out.push_str(&prefix);
            out.push_str(&render_scalar(value).unwrap_or_else(|_| "\"\"".to_owned()));
            out.push_str(".\n");
        }
    }
}

fn render_item_value(value: &Value, indent: usize, out: &mut String) {
    let prefix = format!("{}item is ", " ".repeat(indent));
    match value {
        Value::Object(map) => {
            out.push_str(&prefix);
            out.push_str("record:\n");
            render_record_entries(map, indent + 2, out);
        }
        Value::Array(values) => {
            out.push_str(&prefix);
            out.push_str("list:\n");
            render_list_items(values, indent + 2, out);
        }
        _ => {
            out.push_str(&prefix);
            out.push_str(&render_scalar(value).unwrap_or_else(|_| "\"\"".to_owned()));
            out.push_str(".\n");
        }
    }
}

fn render_scalar(value: &Value) -> Result<String> {
    Ok(match value {
        Value::Null => "none".to_owned(),
        Value::Bool(value) => value.to_string(),
        Value::Number(value) => value.to_string(),
        Value::String(value) => serde_json::to_string(value)?,
        other => bail!("cannot render non-scalar Launchpad Lattice value `{other}`"),
    })
}

fn is_identifier(value: &str) -> bool {
    let mut chars = value.chars();
    let Some(first) = chars.next() else {
        return false;
    };
    if !first.is_ascii_alphabetic() {
        return false;
    }
    chars.all(|ch| ch.is_ascii_alphanumeric() || ch == '_')
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn parses_simple_payload_document() {
        let value = parse_document(
            r#"
lattice 1.

data:
  appId is "hello".
  retries is 3.
  enabled is true.
  architectures is list:
    item is "linux-x86_64".
    item is "linux-arm64".
  extension is record:
    bundle is uri "file:ext/demo.lext".
    sha256 is digest "sha256:abc".
"#,
        )
        .expect("document should parse");

        assert_eq!(value["appId"], "hello");
        assert_eq!(value["retries"], 3);
        assert_eq!(value["enabled"], true);
        assert_eq!(value["architectures"][0], "linux-x86_64");
        assert_eq!(value["extension"]["bundle"], "file:ext/demo.lext");
    }

    #[test]
    fn ignores_non_data_sections_when_data_is_present() {
        let value = parse_document(
            r#"
lattice 1.

document is "org.example.demo".
schema is "org.example/demo/1".

data is Defaults:
  installDir is path "/opt/demo".
"#,
        )
        .expect("document should parse");

        assert_eq!(value["installDir"], "/opt/demo");
    }

    #[test]
    fn renders_round_trip_payloads() {
        let original = serde_json::json!({
            "appId": "hello",
            "enabled": true,
            "threshold": 2.5,
            "labels": {
                "io.launchpad.app/name": "hello"
            },
            "architectures": ["linux-x86_64", "linux-arm64"]
        });
        let rendered = to_string_pretty(&original).expect("render should succeed");
        let reparsed = parse_document(&rendered).expect("rendered document should parse");
        assert_eq!(reparsed, original);
    }

    #[test]
    fn rejects_duplicate_keys() {
        let error = parse_document(
            r#"
lattice 1.

data:
  appId is "hello".
  appId is "duplicate".
"#,
        )
        .expect_err("duplicate keys should fail");
        assert!(error.to_string().contains("duplicate key `appId`"));
    }
}
