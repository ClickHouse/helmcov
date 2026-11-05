package reporters

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/ClickHouse/helmcov/internal/coverage"
)

func TestWriteMarkdownEdges(t *testing.T) {
	t.Parallel()

	halfCovered := coverage.Report{Files: map[string]coverage.FileCoverage{
		"t.yaml": {Lines: map[int]int{1: 1, 2: 0}, Branches: map[string]int{}},
	}}
	empty := coverage.Report{Files: map[string]coverage.FileCoverage{}}

	cases := []struct {
		name    string
		report  coverage.Report
		opts    MarkdownOptions
		want    []string
		notWant []string
	}{
		{
			name:   "blank marker falls back to the default",
			report: empty,
			opts:   MarkdownOptions{CommentMarker: "  "},
			want:   []string{"<!-- helmcov-comment -->\n"},
		},
		{
			name:    "zero threshold hides the threshold row",
			report:  halfCovered,
			opts:    MarkdownOptions{},
			notWant: []string{"| Threshold |"},
		},
		{
			name:   "line rate equal to the threshold meets it",
			report: halfCovered,
			opts:   MarkdownOptions{Threshold: 50},
			want:   []string{"| Threshold | 50% (met) |"},
		},
		{
			name:    "file summary with no files is left out",
			report:  empty,
			opts:    MarkdownOptions{ShowFileSummary: true},
			notWant: []string{"Per-file summary", "<details>"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			if err := WriteMarkdown(tc.report, &buf, tc.opts); err != nil {
				t.Fatal(err)
			}
			for _, s := range tc.want {
				if !strings.Contains(buf.String(), s) {
					t.Errorf("output missing %q:\n%s", s, buf.String())
				}
			}
			for _, s := range tc.notWant {
				if strings.Contains(buf.String(), s) {
					t.Errorf("output should not contain %q:\n%s", s, buf.String())
				}
			}
		})
	}
}

type failingWriter struct{ after int }

func (w *failingWriter) Write(p []byte) (int, error) {
	if w.after <= 0 {
		return 0, errors.New("disk full")
	}
	w.after--
	return len(p), nil
}

func TestWritersReturnWriteErrors(t *testing.T) {
	t.Parallel()

	report := coverage.Report{Files: map[string]coverage.FileCoverage{
		"t.yaml": {Lines: map[int]int{1: 1}, Branches: map[string]int{}},
	}}
	writers := map[string]func(io.Writer) error{
		"coverprofile header": func(w io.Writer) error { return WriteGoCoverProfile(report, w) },
		"cobertura":           func(w io.Writer) error { return WriteCoberturaXML(report, w) },
		"markdown":            func(w io.Writer) error { return WriteMarkdown(report, w, MarkdownOptions{}) },
	}
	for name, write := range writers {
		if err := write(&failingWriter{}); err == nil {
			t.Errorf("%s: want the write error, got nil", name)
		}
	}
	// The header succeeds, so this fails on the first coverage line.
	if err := WriteGoCoverProfile(report, &failingWriter{after: 1}); err == nil {
		t.Error("coverprofile line: want the write error, got nil")
	}
}
