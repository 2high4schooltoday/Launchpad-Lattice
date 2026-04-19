package launchpadlattice

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

const header = "lattice 1."

type logicalLine struct {
	indent int
	text   string
	number int
}

type containerKind int

const (
	recordKind containerKind = iota
	listKind
)

type Error struct {
	Message string
}

func (e *Error) Error() string {
	return e.Message
}

func ParseDocument(content string) (any, error) {
	lines, err := logicalLines(content)
	if err != nil {
		return nil, err
	}
	if len(lines) == 0 {
		return map[string]any{}, nil
	}

	index := 0
	if lines[0].text == "lattice 1." || lines[0].text == "ldf 1." {
		index++
	}

	for index < len(lines) {
		line := lines[index]
		if line.indent != 0 {
			return nil, &Error{
				Message: fmt.Sprintf(
					"expected a top-level Launchpad Lattice statement at line %d",
					line.number,
				),
			}
		}
		if line.text == "data:" {
			value, _, err := parseContainer(lines, index+1, 2, recordKind)
			return value, err
		}
		if kind, ok := blockKindAfterPrefix(line.text, "data is "); ok {
			value, _, err := parseContainer(lines, index+1, 2, kind)
			return value, err
		}
		if strings.HasPrefix(line.text, "data is ") && strings.HasSuffix(line.text, ".") {
			return parseScalar(strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(line.text, "data is "), ".")))
		}
		index++
	}

	return nil, &Error{Message: "Launchpad Lattice document must define a `data` payload"}
}

func ParseBytes(data []byte) (any, error) {
	return ParseDocument(string(data))
}

func Unmarshal[T any](content string) (T, error) {
	var zero T
	value, err := ParseDocument(content)
	if err != nil {
		return zero, err
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return zero, err
	}
	var out T
	if err := json.Unmarshal(raw, &out); err != nil {
		return zero, err
	}
	return out, nil
}

func Marshal(value any) (string, error) {
	normalized, err := normalizeValue(value)
	if err != nil {
		return "", err
	}

	var out strings.Builder
	out.WriteString(header)
	out.WriteString("\n\n")
	switch typed := normalized.(type) {
	case map[string]any:
		out.WriteString("data:\n")
		renderRecordEntries(typed, 2, &out)
	case []any:
		out.WriteString("data is list:\n")
		renderListItems(typed, 2, &out)
	default:
		scalar, err := renderScalar(typed)
		if err != nil {
			return "", err
		}
		out.WriteString("data is ")
		out.WriteString(scalar)
		out.WriteString(".\n")
	}
	return out.String(), nil
}

func MarshalBytes(value any) ([]byte, error) {
	rendered, err := Marshal(value)
	if err != nil {
		return nil, err
	}
	return []byte(rendered), nil
}

func logicalLines(content string) ([]logicalLine, error) {
	rawLines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	lines := make([]logicalLine, 0, len(rawLines))
	for i, rawLine := range rawLines {
		if strings.Contains(rawLine, "\t") {
			return nil, &Error{
				Message: fmt.Sprintf("tabs are not allowed in Launchpad Lattice line %d", i+1),
			}
		}
		stripped := stripComment(rawLine)
		if strings.TrimSpace(stripped) == "" {
			continue
		}
		indent := 0
		for indent < len(stripped) && stripped[indent] == ' ' {
			indent++
		}
		lines = append(lines, logicalLine{
			indent: indent,
			text:   strings.TrimRight(stripped[indent:], " "),
			number: i + 1,
		})
	}
	return lines, nil
}

func stripComment(line string) string {
	var out strings.Builder
	inString := false
	escaped := false
	for _, ch := range line {
		switch {
		case escaped:
			out.WriteRune(ch)
			escaped = false
		case ch == '\\' && inString:
			out.WriteRune(ch)
			escaped = true
		case ch == '"':
			inString = !inString
			out.WriteRune(ch)
		case ch == '#' && !inString:
			return out.String()
		default:
			out.WriteRune(ch)
		}
	}
	return out.String()
}

func parseContainer(lines []logicalLine, index, indent int, kind containerKind) (any, int, error) {
	if kind == recordKind {
		result := map[string]any{}
		for index < len(lines) {
			line := lines[index]
			if line.indent < indent {
				break
			}
			if line.indent != indent {
				return nil, index, &Error{
					Message: fmt.Sprintf(
						"unexpected indentation at line %d: expected %d spaces",
						line.number,
						indent,
					),
				}
			}
			key, value, nextIndex, err := parseRecordEntry(lines, index, indent)
			if err != nil {
				return nil, index, err
			}
			if _, exists := result[key]; exists {
				return nil, index, &Error{
					Message: fmt.Sprintf("duplicate key `%s` at line %d", key, line.number),
				}
			}
			result[key] = value
			index = nextIndex
		}
		return result, index, nil
	}

	values := []any{}
	for index < len(lines) {
		line := lines[index]
		if line.indent < indent {
			break
		}
		if line.indent != indent {
			return nil, index, &Error{
				Message: fmt.Sprintf(
					"unexpected indentation at line %d: expected %d spaces",
					line.number,
					indent,
				),
			}
		}
		value, nextIndex, err := parseListItem(lines, index, indent)
		if err != nil {
			return nil, index, err
		}
		values = append(values, value)
		index = nextIndex
	}
	return values, index, nil
}

func parseRecordEntry(lines []logicalLine, index, indent int) (string, any, int, error) {
	line := lines[index]
	if rest, ok := strings.CutPrefix(line.text, "entry "); ok {
		return parseNamedValueEntry(rest, lines, index, indent)
	}
	return parseNamedValueEntry(line.text, lines, index, indent)
}

func parseNamedValueEntry(text string, lines []logicalLine, index, indent int) (string, any, int, error) {
	line := lines[index]
	name, expr, ok := strings.Cut(text, " is ")
	if ok {
		key, err := parseKey(strings.TrimSpace(name))
		if err != nil {
			return "", nil, index, err
		}
		trimmed := strings.TrimSpace(strings.TrimSuffix(expr, ":"))
		if kind, ok := blockKind(trimmed); ok {
			if !strings.HasSuffix(strings.TrimRightFunc(text, unicode.IsSpace), ":") {
				return "", nil, index, &Error{
					Message: fmt.Sprintf("expected block entry at line %d", line.number),
				}
			}
			value, nextIndex, err := parseContainer(lines, index+1, indent+2, kind)
			return key, value, nextIndex, err
		}
		if !strings.HasSuffix(strings.TrimSpace(expr), ".") {
			return "", nil, index, &Error{
				Message: fmt.Sprintf("expected `.` at line %d", line.number),
			}
		}
		value, err := parseScalar(strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(expr), ".")))
		return key, value, index + 1, err
	}
	if strings.HasSuffix(text, ":") {
		key, err := parseKey(strings.TrimSpace(strings.TrimSuffix(text, ":")))
		if err != nil {
			return "", nil, index, err
		}
		value, nextIndex, err := parseContainer(lines, index+1, indent+2, recordKind)
		return key, value, nextIndex, err
	}
	return "", nil, index, &Error{Message: fmt.Sprintf("invalid record entry at line %d", line.number)}
}

func parseListItem(lines []logicalLine, index, indent int) (any, int, error) {
	line := lines[index]
	rest, ok := strings.CutPrefix(line.text, "item is ")
	if !ok {
		return nil, index, &Error{
			Message: fmt.Sprintf("list items must start with `item is` at line %d", line.number),
		}
	}
	if kind, ok := blockKind(strings.TrimSpace(strings.TrimSuffix(rest, ":"))); ok {
		if !strings.HasSuffix(line.text, ":") {
			return nil, index, &Error{
				Message: fmt.Sprintf("expected block list item at line %d", line.number),
			}
		}
		return parseContainer(lines, index+1, indent+2, kind)
	}
	if !strings.HasSuffix(strings.TrimSpace(rest), ".") {
		return nil, index, &Error{
			Message: fmt.Sprintf("expected `.` at line %d", line.number),
		}
	}
	value, err := parseScalar(strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(rest), ".")))
	return value, index + 1, err
}

func parseKey(raw string) (string, error) {
	if strings.HasPrefix(raw, "\"") {
		value, err := parseString(raw)
		if err != nil {
			return "", err
		}
		stringValue, ok := value.(string)
		if !ok {
			return "", &Error{Message: "key must be a string"}
		}
		return stringValue, nil
	}
	if raw == "" {
		return "", &Error{Message: "empty key is not allowed"}
	}
	return raw, nil
}

func blockKind(expr string) (containerKind, bool) {
	switch {
	case expr == "list" || expr == "set":
		return listKind, true
	case expr == "record" || strings.HasPrefix(expr, "map of "):
		return recordKind, true
	case expr != "" && isWord(expr):
		return recordKind, true
	default:
		return recordKind, false
	}
}

func blockKindAfterPrefix(text, prefix string) (containerKind, bool) {
	if !strings.HasPrefix(text, prefix) || !strings.HasSuffix(text, ":") {
		return recordKind, false
	}
	return blockKind(strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(text, prefix), ":")))
}

func parseScalar(expr string) (any, error) {
	switch expr {
	case "none":
		return nil, nil
	case "true":
		return true, nil
	case "false":
		return false, nil
	}
	if strings.HasPrefix(expr, "\"") {
		return parseString(expr)
	}
	if tagged, ok, err := parseTaggedString(expr); ok || err != nil {
		return tagged, err
	}
	if intValue, err := strconv.ParseInt(expr, 10, 64); err == nil {
		return intValue, nil
	}
	if uintValue, err := strconv.ParseUint(expr, 10, 64); err == nil {
		return uintValue, nil
	}
	if floatValue, err := strconv.ParseFloat(expr, 64); err == nil && !math.IsInf(floatValue, 0) && !math.IsNaN(floatValue) {
		return floatValue, nil
	}
	return expr, nil
}

func parseTaggedString(expr string) (string, bool, error) {
	before, rest, ok := strings.Cut(expr, " ")
	_ = before
	if !ok {
		return "", false, nil
	}
	rest = strings.TrimSpace(rest)
	if strings.HasPrefix(rest, "\"") {
		value, err := parseString(rest)
		if err != nil {
			return "", true, err
		}
		return value.(string), true, nil
	}
	_, tail, ok := strings.Cut(rest, " ")
	if ok {
		tail = strings.TrimSpace(tail)
		if strings.HasPrefix(tail, "\"") {
			value, err := parseString(tail)
			if err != nil {
				return "", true, err
			}
			return value.(string), true, nil
		}
	}
	return "", false, nil
}

func parseString(expr string) (any, error) {
	var value any
	if err := json.Unmarshal([]byte(expr), &value); err != nil {
		return nil, &Error{Message: err.Error()}
	}
	return value, nil
}

func renderRecordEntries(record map[string]any, indent int, out *strings.Builder) {
	keys := make([]string, 0, len(record))
	for key := range record {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		renderNamedValue(key, record[key], indent, out)
	}
}

func renderListItems(values []any, indent int, out *strings.Builder) {
	for _, value := range values {
		renderItemValue(value, indent, out)
	}
}

func renderNamedValue(name string, value any, indent int, out *strings.Builder) {
	prefix := strings.Repeat(" ", indent)
	if isIdentifier(name) {
		prefix += name + " is "
	} else {
		raw, _ := json.Marshal(name)
		prefix += "entry " + string(raw) + " is "
	}

	switch typed := value.(type) {
	case map[string]any:
		out.WriteString(prefix)
		out.WriteString("record:\n")
		renderRecordEntries(typed, indent+2, out)
	case []any:
		out.WriteString(prefix)
		out.WriteString("list:\n")
		renderListItems(typed, indent+2, out)
	default:
		scalar, err := renderScalar(typed)
		if err != nil {
			scalar = "\"\""
		}
		out.WriteString(prefix)
		out.WriteString(scalar)
		out.WriteString(".\n")
	}
}

func renderItemValue(value any, indent int, out *strings.Builder) {
	prefix := strings.Repeat(" ", indent) + "item is "
	switch typed := value.(type) {
	case map[string]any:
		out.WriteString(prefix)
		out.WriteString("record:\n")
		renderRecordEntries(typed, indent+2, out)
	case []any:
		out.WriteString(prefix)
		out.WriteString("list:\n")
		renderListItems(typed, indent+2, out)
	default:
		scalar, err := renderScalar(typed)
		if err != nil {
			scalar = "\"\""
		}
		out.WriteString(prefix)
		out.WriteString(scalar)
		out.WriteString(".\n")
	}
}

func renderScalar(value any) (string, error) {
	switch typed := value.(type) {
	case nil:
		return "none", nil
	case bool:
		if typed {
			return "true", nil
		}
		return "false", nil
	case string:
		raw, err := json.Marshal(typed)
		return string(raw), err
	case int:
		return strconv.Itoa(typed), nil
	case int8, int16, int32, int64:
		return fmt.Sprintf("%d", typed), nil
	case uint, uint8, uint16, uint32, uint64:
		return fmt.Sprintf("%d", typed), nil
	case float32:
		return strconv.FormatFloat(float64(typed), 'f', -1, 32), nil
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64), nil
	case json.Number:
		return typed.String(), nil
	default:
		return "", &Error{Message: fmt.Sprintf("cannot render non-scalar Launchpad Lattice value `%v`", value)}
	}
}

func isIdentifier(value string) bool {
	if value == "" {
		return false
	}
	for i, ch := range value {
		if i == 0 {
			if !('A' <= ch && ch <= 'Z' || 'a' <= ch && ch <= 'z') {
				return false
			}
			continue
		}
		if !('A' <= ch && ch <= 'Z' || 'a' <= ch && ch <= 'z' || '0' <= ch && ch <= '9' || ch == '_') {
			return false
		}
	}
	return true
}

func isWord(value string) bool {
	if value == "" {
		return false
	}
	for _, ch := range value {
		if !('A' <= ch && ch <= 'Z' || 'a' <= ch && ch <= 'z' || '0' <= ch && ch <= '9' || ch == '_') {
			return false
		}
	}
	return true
}

func normalizeValue(value any) (any, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var normalized any
	if err := decoder.Decode(&normalized); err != nil {
		return nil, err
	}
	return convertJSONNumbers(normalized), nil
}

func convertJSONNumbers(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		out := map[string]any{}
		for key, inner := range typed {
			out[key] = convertJSONNumbers(inner)
		}
		return out
	case []any:
		out := make([]any, len(typed))
		for i, inner := range typed {
			out[i] = convertJSONNumbers(inner)
		}
		return out
	case json.Number:
		if strings.ContainsAny(typed.String(), ".eE") {
			value, err := typed.Float64()
			if err == nil {
				return value
			}
			return typed.String()
		}
		if value, err := typed.Int64(); err == nil {
			return value
		}
		return typed.String()
	default:
		return value
	}
}
