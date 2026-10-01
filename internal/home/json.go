package home

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"sort"
	"strconv"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

// Field is an ordered JSON object's own property.
type Field struct {
	Key   string
	Value any
}

// Object preserves JavaScript property order in answers, hooks and run records.
type Object []Field

// O constructs an object from alternating string keys and JSON values.
func O(kv ...any) Object {
	o := Object{}
	for i := 0; i < len(kv); i += 2 {
		o.Set(kv[i].(string), kv[i+1])
	}
	return o
}

// Get returns an own property, or nil when absent.
func (o Object) Get(key string) any {
	for _, f := range o {
		if f.Key == key {
			return f.Value
		}
	}
	return nil
}

// Has distinguishes absent properties from JSON null.
func (o Object) Has(key string) bool {
	for _, f := range o {
		if f.Key == key {
			return true
		}
	}
	return false
}

// Set replaces a property without changing its position, or appends it.
func (o *Object) Set(key string, v any) {
	for i := range *o {
		if (*o)[i].Key == key {
			(*o)[i].Value = v
			return
		}
	}
	*o = append(*o, Field{key, v})
}

// Delete removes an own property.
func (o *Object) Delete(key string) {
	for i, f := range *o {
		if f.Key == key {
			*o = append((*o)[:i], (*o)[i+1:]...)
			return
		}
	}
}

// Clone copies the properties for task updates without changing the recorded task.
func (o Object) Clone() Object { return append(Object{}, o...) }

// String implements the JSON-value string conversions used by task ids and options.
func String(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case nil:
		return "null"
	case bool:
		if x {
			return "true"
		}
		return "false"
	case float64:
		if math.IsNaN(x) {
			return "NaN"
		}
		if math.IsInf(x, 1) {
			return "Infinity"
		}
		if math.IsInf(x, -1) {
			return "-Infinity"
		}
		return numberText(x)
	case int:
		return strconv.Itoa(x)
	case Object:
		return "[object Object]"
	case []any:
		parts := []string{}
		for _, a := range x {
			s := ""
			if a != nil {
				s = String(a)
			}
			parts = append(parts, s)
		}
		return strings.Join(parts, ",")
	}
	return fmt.Sprint(v)
}

// S returns a string property, or the empty string.
func (o Object) S(k string) string { s, _ := o.Get(k).(string); return s }

// N returns a numeric property, or zero.
func (o Object) N(k string) float64 { return Number(o.Get(k)) }

// B returns JavaScript truthiness of a property.
func (o Object) B(k string) bool { return Truth(o.Get(k)) }

// Number converts JSON numbers and numeric option strings like JavaScript Number.
func Number(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case int:
		return float64(x)
	case nil:
		return 0
	case bool:
		if x {
			return 1
		}
		return 0
	case string:
		s := Trim(x)
		if s == "" {
			return 0
		}
		if strings.Contains(s, "_") {
			return math.NaN()
		}
		if len(s) > 2 && s[0] == '0' && strings.ContainsRune("xXbBoO", rune(s[1])) {
			base := 16
			if s[1] == 'b' || s[1] == 'B' {
				base = 2
			}
			if s[1] == 'o' || s[1] == 'O' {
				base = 8
			}
			if n, ok := new(big.Int).SetString(s[2:], base); ok && n.Sign() >= 0 && s[2] != '+' && s[2] != '-' {
				f, _ := new(big.Float).SetInt(n).Float64()
				return f
			}
		}
		if strings.ContainsAny(s, "_pP") || strings.EqualFold(strings.TrimLeft(s, "+-"), "inf") || (strings.EqualFold(strings.TrimLeft(s, "+-"), "infinity") && strings.TrimLeft(s, "+-") != "Infinity") {
			return math.NaN()
		}
		n, e := strconv.ParseFloat(s, 64)
		if e == nil || math.IsInf(n, 0) {
			return n
		}
	}
	return math.NaN()
}

// Truth follows JSON's JavaScript truthiness, including truthy empty objects and arrays.
func Truth(v any) bool {
	switch x := v.(type) {
	case nil:
		return false
	case bool:
		return x
	case string:
		return x != ""
	case float64:
		return x != 0 && !math.IsNaN(x)
	case int:
		return x != 0
	}
	return true
}

// Kind returns the JSON type name used in validation errors.
func Kind(v any) string {
	switch v.(type) {
	case nil:
		return "null"
	case bool:
		return "boolean"
	case string:
		return "string"
	case float64, int:
		return "number"
	case []any, []Object:
		return "array"
	default:
		return "object"
	}
}

// numberText emits the original runtime's JSON number spelling and exponent thresholds.
func numberText(n float64) string {
	if math.IsNaN(n) || math.IsInf(n, 0) {
		return "null"
	}
	if n == 0 {
		return "0"
	}
	fmtc := byte('f')
	if math.Abs(n) >= 1e21 || math.Abs(n) < 1e-6 {
		fmtc = 'e'
	}
	s := strconv.FormatFloat(n, fmtc, -1, 64)
	s = strings.ReplaceAll(s, "e-0", "e-")
	s = strings.ReplaceAll(s, "e+0", "e+")
	return s
}

// Fixed formats the exact binary value using JavaScript's decimal rounding (ties away from zero).
func Fixed(n float64, digits int) string {
	if math.IsInf(n, 0) || math.IsNaN(n) || math.Abs(n) >= 1e21 {
		return String(n)
	}
	negative := n < 0
	r := new(big.Rat).SetFloat64(math.Abs(n))
	scale := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(digits)), nil)
	r.Mul(r, new(big.Rat).SetInt(scale))
	q, rem := new(big.Int), new(big.Int)
	q.QuoRem(r.Num(), r.Denom(), rem)
	if rem.Lsh(rem, 1).Cmp(r.Denom()) >= 0 {
		q.Add(q, big.NewInt(1))
	}
	s := q.String()
	if digits > 0 {
		if len(s) <= digits {
			s = strings.Repeat("0", digits+1-len(s)) + s
		}
		s = s[:len(s)-digits] + "." + s[len(s)-digits:]
	}
	if negative {
		s = "-" + s
	}
	return s
}

// Space is the JavaScript whitespace set used by prompt, option and report trimming.
func Space(r rune) bool {
	return r == ' ' || (r >= '\t' && r <= '\r') || r == '\u00a0' || r == '\u1680' || (r >= '\u2000' && r <= '\u200a') || r == '\u2028' || r == '\u2029' || r == '\u202f' || r == '\u205f' || r == '\u3000' || r == '\ufeff'
}

// Trim removes the same leading and trailing whitespace as the original CLI.
func Trim(s string) string { return strings.TrimFunc(s, Space) }

// TrimEnd preserves leading answer whitespace while removing the original trailing whitespace set.
func TrimEnd(s string) string { return strings.TrimRightFunc(s, Space) }

// UTF8 decodes byte streams with the original runtime's replacement of malformed subsequences.
func UTF8(b []byte) string {
	var out strings.Builder
	for len(b) > 0 {
		r, n := utf8.DecodeRune(b)
		if r != utf8.RuneError || n > 1 {
			out.Write(b[:n])
			b = b[n:]
			continue
		}
		n = 1
		need, low, high := 0, byte(0x80), byte(0xbf)
		switch {
		case b[0] >= 0xc2 && b[0] <= 0xdf:
			need = 2
		case b[0] >= 0xe0 && b[0] <= 0xef:
			need = 3
			if b[0] == 0xe0 {
				low = 0xa0
			}
			if b[0] == 0xed {
				high = 0x9f
			}
		case b[0] >= 0xf0 && b[0] <= 0xf4:
			need = 4
			if b[0] == 0xf0 {
				low = 0x90
			}
			if b[0] == 0xf4 {
				high = 0x8f
			}
		}
		if need > 0 && len(b) > 1 && b[1] >= low && b[1] <= high {
			n = 2
			for n < need && n < len(b) && b[n] >= 0x80 && b[n] <= 0xbf {
				n++
			}
		}
		out.WriteRune(utf8.RuneError)
		b = b[n:]
	}
	return out.String()
}

// quote preserves HTML, Unicode separators and lone UTF-16 surrogates in JSON output.
func quote(s string) string {
	var b bytes.Buffer
	b.WriteByte('"')
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case '"', '\\':
			b.WriteByte('\\')
			b.WriteByte(c)
		case '\b':
			b.WriteString(`\b`)
		case '\f':
			b.WriteString(`\f`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			if c < 32 {
				fmt.Fprintf(&b, `\u%04x`, c)
			} else if c == 0xed && i+2 < len(s) && s[i+1] >= 0xa0 && s[i+1] <= 0xbf && s[i+2] >= 0x80 && s[i+2] <= 0xbf {
				unit := uint16(c&15)<<12 | uint16(s[i+1]&63)<<6 | uint16(s[i+2]&63)
				fmt.Fprintf(&b, `\u%04x`, unit)
				i += 2
			} else {
				b.WriteByte(c)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}

// stringValue decodes validated JSON strings, retaining lone surrogates as WTF-8 for serialization.
func stringValue(raw string) string {
	var b strings.Builder
	for i := 1; i < len(raw)-1; i++ {
		if raw[i] != '\\' {
			b.WriteByte(raw[i])
			continue
		}
		i++
		switch raw[i] {
		case 'b':
			b.WriteByte('\b')
		case 'f':
			b.WriteByte('\f')
		case 'n':
			b.WriteByte('\n')
		case 'r':
			b.WriteByte('\r')
		case 't':
			b.WriteByte('\t')
		case 'u':
			unit, _ := strconv.ParseUint(raw[i+1:i+5], 16, 16)
			i += 4
			if unit >= 0xd800 && unit <= 0xdbff && i+6 < len(raw)-1 && raw[i+1:i+3] == `\u` {
				low, _ := strconv.ParseUint(raw[i+3:i+7], 16, 16)
				if low >= 0xdc00 && low <= 0xdfff {
					b.WriteRune(utf16.DecodeRune(rune(unit), rune(low)))
					i += 6
					continue
				}
			}
			if unit >= 0xd800 && unit <= 0xdfff {
				b.WriteByte(0xe0 | byte(unit>>12))
				b.WriteByte(0x80 | byte((unit>>6)&63))
				b.WriteByte(0x80 | byte(unit&63))
			} else {
				b.WriteRune(rune(unit))
			}
		default:
			b.WriteByte(raw[i])
		}
	}
	return b.String()
}

// keys orders array-index properties numerically before other insertion-ordered properties.
func keys(o Object) Object {
	indices, other := Object{}, Object{}
	for _, f := range o {
		n, e := strconv.ParseUint(f.Key, 10, 32)
		if e == nil && n < math.MaxUint32 && strconv.FormatUint(n, 10) == f.Key {
			indices = append(indices, f)
		} else {
			other = append(other, f)
		}
	}
	sort.Slice(indices, func(i, j int) bool { return Number(indices[i].Key) < Number(indices[j].Key) })
	return append(indices, other...)
}

// JSON serializes values with the original CLI's property order and two-space indentation.
func JSON(v any, pretty bool) string {
	var encode func(any, int) string
	encode = func(v any, depth int) string {
		indent, sep, colon := "", ",", ":"
		if pretty {
			indent = strings.Repeat("  ", depth+1)
			sep = ",\n"
			colon = ": "
		}
		parts := []string{}
		open, close := "", ""
		switch x := v.(type) {
		case Object:
			open, close = "{", "}"
			for _, f := range keys(x) {
				parts = append(parts, indent+quote(f.Key)+colon+encode(f.Value, depth+1))
			}
		case []any:
			open, close = "[", "]"
			for _, a := range x {
				parts = append(parts, indent+encode(a, depth+1))
			}
		case []Object:
			open, close = "[", "]"
			for _, a := range x {
				if a == nil {
					parts = append(parts, indent+"null")
				} else {
					parts = append(parts, indent+encode(a, depth+1))
				}
			}
		case string:
			return quote(x)
		case float64:
			return numberText(x)
		case int:
			return strconv.Itoa(x)
		case bool:
			if x {
				return "true"
			}
			return "false"
		case nil:
			return "null"
		default:
			b, _ := json.Marshal(v)
			return string(b)
		}
		if len(parts) == 0 {
			return open + close
		}
		if pretty {
			return open + "\n" + strings.Join(parts, sep) + "\n" + strings.Repeat("  ", depth) + close
		}
		return open + strings.Join(parts, sep) + close
	}
	return encode(v, 0)
}

// ParseJSON decodes JSON preserving own-property order and the existing parser's diagnostics.
func ParseJSON(text string) (any, error) {
	p := parser{s: text}
	v, e := p.value()
	if e != nil {
		return nil, e
	}
	p.space()
	if p.i < len(text) {
		return nil, p.at("Unexpected non-whitespace character after JSON")
	}
	return v, nil
}

type parser struct {
	s string
	i int
}

// space consumes only JSON whitespace, leaving other whitespace as invalid input.
func (p *parser) space() {
	for p.i < len(p.s) && strings.ContainsRune(" \n\r\t", rune(p.s[p.i])) {
		p.i++
	}
}

// at reports a parser failure with UTF-16 position, line and column.
func (p *parser) at(why string) error {
	prefix := p.s[:p.i]
	pos := len(utf16.Encode([]rune(prefix)))
	line := strings.Count(prefix, "\n") + 1
	last := strings.LastIndex(prefix, "\n")
	col := len(utf16.Encode([]rune(prefix[last+1:]))) + 1
	if !strings.HasSuffix(why, "JSON") {
		why += " in JSON"
	}
	return fmt.Errorf("%s at position %d (line %d column %d)", why, pos, line, col)
}

// unexpected reports an invalid token with the original input excerpt.
func (p *parser) unexpected() error {
	if p.i >= len(p.s) {
		return fmt.Errorf("Unexpected end of JSON input")
	}
	s := p.s
	if s == "undefined" || s == "NaN" || s == "Infinity" || s == "[object Object]" {
		return fmt.Errorf(`"%s" is not valid JSON`, s)
	}
	shown := `"` + s + `"`
	if len(s) > 20 {
		if p.i < 10 {
			shown = `"` + s[:10] + `"...`
		} else if p.i >= len(s)-10 {
			shown = `..."` + s[len(s)-10:] + `"`
		} else {
			shown = `..."` + s[p.i-10:p.i+10] + `"...`
		}
	}
	return fmt.Errorf("Unexpected token '%c', %s is not valid JSON", p.s[p.i], shown)
}

// str validates JSON escapes and returns the decoded string without losing lone surrogates.
func (p *parser) str() (string, error) {
	start := p.i
	p.i++
	for p.i < len(p.s) {
		c := p.s[p.i]
		if c == '"' {
			p.i++
			return stringValue(p.s[start:p.i]), nil
		}
		if c < 32 {
			return "", p.at("Bad control character in string literal")
		}
		if c == '\\' {
			p.i++
			if p.i >= len(p.s) {
				break
			}
			if p.s[p.i] == 'u' {
				for j := 0; j < 4; j++ {
					p.i++
					if p.i >= len(p.s) || !strings.ContainsRune("0123456789abcdefABCDEF", rune(p.s[p.i])) {
						return "", p.at("Bad Unicode escape")
					}
				}
			} else if !strings.ContainsRune(`"\/bfnrt`, rune(p.s[p.i])) {
				return "", p.at("Bad escaped character")
			}
		}
		p.i++
	}
	return "", p.at("Unterminated string")
}

// value parses one ordered JSON value and preserves the original syntax diagnostics.
func (p *parser) value() (any, error) {
	p.space()
	if p.i >= len(p.s) {
		return nil, p.unexpected()
	}
	switch p.s[p.i] {
	case '"':
		return p.str()
	case '{':
		p.i++
		p.space()
		o := Object{}
		if p.i < len(p.s) && p.s[p.i] == '}' {
			p.i++
			return o, nil
		}
		for {
			if p.i >= len(p.s) || p.s[p.i] != '"' {
				return nil, p.at("Expected property name or '}'")
			}
			k, e := p.str()
			if e != nil {
				return nil, e
			}
			p.space()
			if p.i >= len(p.s) || p.s[p.i] != ':' {
				return nil, p.at("Expected ':' after property name")
			}
			p.i++
			v, e := p.value()
			if e != nil {
				return nil, e
			}
			o.Set(k, v)
			p.space()
			if p.i < len(p.s) && p.s[p.i] == '}' {
				p.i++
				return keys(o), nil
			}
			if p.i >= len(p.s) || p.s[p.i] != ',' {
				return nil, p.at("Expected ',' or '}' after property value")
			}
			p.i++
			p.space()
			if p.i >= len(p.s) || p.s[p.i] != '"' {
				return nil, p.at("Expected double-quoted property name")
			}
		}
	case '[':
		p.i++
		p.space()
		a := []any{}
		if p.i < len(p.s) && p.s[p.i] == ']' {
			p.i++
			return a, nil
		}
		for {
			v, e := p.value()
			if e != nil {
				return nil, e
			}
			a = append(a, v)
			p.space()
			if p.i < len(p.s) && p.s[p.i] == ']' {
				p.i++
				return a, nil
			}
			if p.i >= len(p.s) || p.s[p.i] != ',' {
				return nil, p.at("Expected ',' or ']' after array element")
			}
			p.i++
		}
	case 't', 'f', 'n':
		word := "true"
		var v any = true
		if p.s[p.i] == 'f' {
			word = "false"
			v = false
		} else if p.s[p.i] == 'n' {
			word = "null"
			v = nil
		}
		for _, c := range word {
			if p.i >= len(p.s) || p.s[p.i] != byte(c) {
				return nil, p.unexpected()
			}
			p.i++
		}
		return v, nil
	default:
		if !strings.ContainsRune("-0123456789", rune(p.s[p.i])) {
			return nil, p.unexpected()
		}
		start := p.i
		if p.s[p.i] == '-' {
			p.i++
			if p.i >= len(p.s) || p.s[p.i] < '0' || p.s[p.i] > '9' {
				return nil, p.at("No number after minus sign")
			}
		}
		if p.s[p.i] == '0' {
			p.i++
			if p.i < len(p.s) && p.s[p.i] >= '0' && p.s[p.i] <= '9' {
				return nil, p.at("Unexpected number")
			}
		} else {
			for p.i < len(p.s) && p.s[p.i] >= '0' && p.s[p.i] <= '9' {
				p.i++
			}
		}
		if p.i < len(p.s) && p.s[p.i] == '.' {
			p.i++
			if p.i >= len(p.s) || p.s[p.i] < '0' || p.s[p.i] > '9' {
				return nil, p.at("Unterminated fractional number")
			}
			for p.i < len(p.s) && p.s[p.i] >= '0' && p.s[p.i] <= '9' {
				p.i++
			}
		}
		if p.i < len(p.s) && (p.s[p.i] == 'e' || p.s[p.i] == 'E') {
			p.i++
			if p.i < len(p.s) && (p.s[p.i] == '+' || p.s[p.i] == '-') {
				p.i++
			}
			if p.i >= len(p.s) || p.s[p.i] < '0' || p.s[p.i] > '9' {
				return nil, p.at("Exponent part is missing a number")
			}
			for p.i < len(p.s) && p.s[p.i] >= '0' && p.s[p.i] <= '9' {
				p.i++
			}
		}
		n, _ := strconv.ParseFloat(p.s[start:p.i], 64)
		return n, nil
	}
}
