package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"go-web-crawler/internal/crawler"
)

var sample = []crawler.PageResult{
	{URL: "https://example.com/", Depth: 0, StatusCode: 200, Title: "Home",
		InternalLinks: []string{"https://example.com/a", "https://example.com/b"},
		ExternalLinks: []string{"https://github.com/"}},
	{URL: "https://example.com/a", Depth: 1, StatusCode: 404, InternalLinks: []string{}, ExternalLinks: []string{}},
	{URL: "https://example.com/b", Depth: 1, Error: "context deadline exceeded"},
}

func TestTextOutput(t *testing.T) {
	var buf bytes.Buffer
	p := NewText(&buf)
	for _, r := range sample {
		if err := p.Page(r); err != nil {
			t.Fatal(err)
		}
	}
	if err := p.Finish(); err != nil {
		t.Fatal(err)
	}

	want := strings.Join([]string{
		`200  d=0  https://example.com/  "Home"  int=2 ext=1`,
		`404  d=1  https://example.com/a  ""  int=0 ext=0`,
		`ERR  d=1  https://example.com/b  context deadline exceeded`,
		``,
		`crawled 3 pages, 1 errors`,
		``,
	}, "\n")
	if got := buf.String(); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestTextStreamsBeforeFinish(t *testing.T) {
	var buf bytes.Buffer
	p := NewText(&buf)
	p.Page(sample[0])
	if buf.Len() == 0 {
		t.Error("text printer should write each page immediately")
	}
}

func TestJSONOutput(t *testing.T) {
	var buf bytes.Buffer
	p := NewJSON(&buf)
	for _, r := range sample {
		p.Page(r)
	}
	if buf.Len() != 0 {
		t.Error("JSON printer must not write before Finish")
	}
	if err := p.Finish(); err != nil {
		t.Fatal(err)
	}

	var got []crawler.PageResult
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, buf.String())
	}
	if len(got) != len(sample) || got[0].Title != "Home" || got[2].Error == "" {
		t.Errorf("unexpected decoded results: %+v", got)
	}
}

func TestJSONEmptyIsArray(t *testing.T) {
	var buf bytes.Buffer
	p := NewJSON(&buf)
	p.Finish()
	if strings.TrimSpace(buf.String()) != "[]" {
		t.Errorf("empty output = %q, want []", buf.String())
	}
}
