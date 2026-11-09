// Package format builds Telegram HTML messages safely.
//
// HTML is a struct, not a string type, so an untyped string literal cannot
// become HTML by accident: every piece of text goes through Text (escaped)
// or a tag helper. Telegram rejects the whole message if it contains an
// unescaped <, > or &, e.g. from a container name or an error.
package format

import (
	"fmt"
	"html"
	"reflect"
	"strings"
)

// HTML is a fragment of Telegram HTML (parse_mode=HTML).
type HTML struct{ s string }

// String returns the markup.
func (h HTML) String() string { return h.s }

// IsZero reports whether h is empty.
func (h HTML) IsZero() bool { return h.s == "" }

// Text escapes plain text.
func Text(s string) HTML { return HTML{html.EscapeString(s)} }

func Bold(s string) HTML   { return HTML{"<b>" + html.EscapeString(s) + "</b>"} }
func Italic(s string) HTML { return HTML{"<i>" + html.EscapeString(s) + "</i>"} }
func Code(s string) HTML   { return HTML{"<code>" + html.EscapeString(s) + "</code>"} }

// Fmt formats like fmt.Sprintf with a trusted markup template: HTML
// arguments are inserted as is, all other arguments are escaped.
// The template must be a constant written in code, never user input.
func Fmt(template string, args ...any) HTML {
	conv := make([]any, len(args))
	for i, a := range args {
		switch v := a.(type) {
		case HTML:
			conv[i] = rawArg(v.s)
		case string:
			conv[i] = rawArg(html.EscapeString(v))
		case error:
			conv[i] = rawArg(html.EscapeString(v.Error()))
		case fmt.Stringer:
			conv[i] = rawArg(html.EscapeString(v.String()))
		default:
			if isPlainValue(a) {
				conv[i] = a // numbers and bools cannot contain markup
			} else {
				conv[i] = rawArg(html.EscapeString(fmt.Sprint(a)))
			}
		}
	}
	return HTML{fmt.Sprintf(template, conv...)}
}

func isPlainValue(a any) bool {
	switch reflect.ValueOf(a).Kind() {
	case reflect.Bool,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return true
	}
	return false
}

// rawArg makes %s/%v print an already-escaped string without re-quoting.
type rawArg string

func (r rawArg) String() string { return string(r) }

// Join concatenates fragments with sep (escaped).
func Join(parts []HTML, sep string) HTML {
	var b strings.Builder
	esc := html.EscapeString(sep)
	for i, p := range parts {
		if i > 0 {
			b.WriteString(esc)
		}
		b.WriteString(p.s)
	}
	return HTML{b.String()}
}

// Lines joins non-empty fragments with newlines.
func Lines(parts ...HTML) HTML {
	var kept []HTML
	for _, p := range parts {
		if !p.IsZero() {
			kept = append(kept, p)
		}
	}
	return Join(kept, "\n")
}

// Concat concatenates fragments.
func Concat(parts ...HTML) HTML { return Join(parts, "") }
