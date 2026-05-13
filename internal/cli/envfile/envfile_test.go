package envfile

import (
	"strings"
	"testing"
)

func TestParse_BasicShape(t *testing.T) {
	src := `# A comment
FOO=bar
EMPTY=
QUOTED="hello world"
SQ='single'

# @secret
DB_PASSWORD=hunter2

# unrelated comment
API_KEY=public
`
	got, err := Parse(strings.NewReader(src))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	want := []Entry{
		{Line: 2, Name: "FOO", Value: "bar"},
		{Line: 3, Name: "EMPTY", Value: ""},
		{Line: 4, Name: "QUOTED", Value: "hello world"},
		{Line: 5, Name: "SQ", Value: "single"},
		{Line: 8, Name: "DB_PASSWORD", Value: "hunter2", IsSecret: true},
		{Line: 11, Name: "API_KEY", Value: "public"},
	}
	if len(got) != len(want) {
		t.Fatalf("len=%d, want %d (got=%+v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("entry %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestParse_SecretMarkerVariants(t *testing.T) {
	cases := map[string]bool{
		"# @secret\nX=1\n":              true,
		"#@secret\nX=1\n":               true,
		"# @Secret reason text\nX=1\n":  true,
		"# @SECRET\nX=1\n":              true,
		"# @secretly\nX=1\n":            false, // not exactly the marker
		"# @secret\n\nX=1\n":            false, // blank line resets it
		"# @secret\n# rationale\nX=1\n": true,  // intervening comment is OK
		"X=1\n":                         false,
	}
	for src, want := range cases {
		t.Run(src, func(t *testing.T) {
			got, err := Parse(strings.NewReader(src))
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			if len(got) != 1 {
				t.Fatalf("entries=%d, want 1", len(got))
			}
			if got[0].IsSecret != want {
				t.Errorf("IsSecret=%v, want %v (src=%q)", got[0].IsSecret, want, src)
			}
		})
	}
}

func TestParse_SecretMarkerDoesNotBleed(t *testing.T) {
	src := "# @secret\nFIRST=a\nSECOND=b\n"
	got, err := Parse(strings.NewReader(src))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if !got[0].IsSecret {
		t.Errorf("FIRST should be secret")
	}
	if got[1].IsSecret {
		t.Errorf("SECOND should not be secret — marker only applies to next var")
	}
}

func TestParse_ExportPrefix(t *testing.T) {
	got, err := Parse(strings.NewReader("export FOO=bar\n"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(got) != 1 || got[0].Name != "FOO" || got[0].Value != "bar" {
		t.Errorf("got %+v", got)
	}
}

func TestParse_InlineCommentStripped(t *testing.T) {
	got, err := Parse(strings.NewReader("FOO=bar # trailing\n"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got[0].Value != "bar" {
		t.Errorf("value=%q, want %q", got[0].Value, "bar")
	}
}

func TestParse_InlineCommentNotStrippedInsideQuotes(t *testing.T) {
	got, err := Parse(strings.NewReader(`FOO="a # b"` + "\n"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got[0].Value != "a # b" {
		t.Errorf("value=%q, want %q", got[0].Value, "a # b")
	}
}

func TestParse_MalformedLine(t *testing.T) {
	_, err := Parse(strings.NewReader("just-some-words\n"))
	if err == nil {
		t.Fatal("expected error for line without =")
	}
}
