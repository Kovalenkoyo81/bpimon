package format

import "testing"

func TestEscaping(t *testing.T) {
	cases := map[HTML]string{
		Text("a <b> & c"):                         "a &lt;b&gt; &amp; c",
		Bold("x<y"):                               "<b>x&lt;y</b>",
		Code("a&b"):                               "<code>a&amp;b</code>",
		Italic("<i>"):                             "<i>&lt;i&gt;</i>",
		Fmt("<b>%s</b> %d%%", "<x>", 5):           "<b>&lt;x&gt;</b> 5%",
		Fmt("%s · %s", Bold("a"), "b&c"):          "<b>a</b> · b&amp;c",
		Lines(Text("a"), HTML{}, Text("b")):       "a\nb",
		Join([]HTML{Text("a"), Text("b")}, " & "): "a &amp; b",
		Concat(Bold("a"), Text("<")):              "<b>a</b>&lt;",
	}
	for got, want := range cases {
		if got.String() != want {
			t.Errorf("got %q, want %q", got.String(), want)
		}
	}
}

func TestFmtEscapesNonScalars(t *testing.T) {
	if got := Fmt("%v", []string{"<x>"}).String(); got != "[&lt;x&gt;]" {
		t.Fatalf("got %q", got)
	}
}

func TestFmtFloat(t *testing.T) {
	if got := Fmt("%.1f%%", 9.04).String(); got != "9.0%" {
		t.Fatalf("got %q", got)
	}
}
