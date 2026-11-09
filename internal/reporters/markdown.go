package reporters

import (
	"fmt"
	"io"
	"strings"

	"github.com/ClickHouse/helmcov/internal/coverage"
)

type MarkdownOptions struct {
	Threshold         float64
	BranchThreshold   float64
	CommentMarker     string
	ShowFileSummary   bool
	IncludeTplSources bool
	// Tests is the coverage from test renders alone, without generated scenarios. Nil omits it.
	Tests *coverage.Report
}

func WriteMarkdown(report coverage.Report, writer io.Writer, opts MarkdownOptions) error {
	marker := strings.TrimSpace(opts.CommentMarker)
	if marker == "" {
		marker = "helmcov-comment"
	}

	lineRate := report.LineRate() * 100
	branchRate := report.BranchRate() * 100
	files := markdownFiles(report, opts.IncludeTplSources)

	var b strings.Builder
	fmt.Fprintf(&b, "<!-- %s -->\n\n", marker)
	b.WriteString("## Helm template coverage\n\n")
	if opts.Tests == nil {
		b.WriteString("| Metric | Value |\n")
		b.WriteString("|--------|------:|\n")
		fmt.Fprintf(&b, "| Line coverage | %.2f%% |\n", lineRate)
		fmt.Fprintf(&b, "| Branch coverage | %.2f%% |\n", branchRate)
	} else {
		b.WriteString("| Metric | Value | Tests only |\n")
		b.WriteString("|--------|------:|-----------:|\n")
		fmt.Fprintf(&b, "| Line coverage | %.2f%% | %.2f%% |\n", lineRate, opts.Tests.LineRate()*100)
		fmt.Fprintf(&b, "| Branch coverage | %.2f%% | %.2f%% |\n", branchRate, opts.Tests.BranchRate()*100)
	}
	if opts.Threshold > 0 {
		fmt.Fprintf(&b, "| Line threshold | %.0f%% %s |\n", opts.Threshold, thresholdStatus(lineRate, opts.Threshold))
	}
	if opts.BranchThreshold > 0 {
		fmt.Fprintf(&b, "| Branch threshold | %.0f%% %s |\n", opts.BranchThreshold, thresholdStatus(branchRate, opts.BranchThreshold))
	}
	b.WriteString("\n")
	if opts.Tests != nil {
		b.WriteString("Tests only counts what helm-unittest tests render. The rest comes from generated value scenarios, which no test asserts on.\n\n")
	}

	if opts.ShowFileSummary && len(files) > 0 {
		b.WriteString("### Per-file summary\n\n")
		b.WriteString("| Template | Line | Branch |\n")
		b.WriteString("|----------|-----:|-----:|\n")
		for _, file := range files {
			fmt.Fprintf(
				&b,
				"| %s | %.2f%% | %.2f%% |\n",
				file,
				report.FileLineRate(file)*100,
				report.FileBranchRate(file)*100,
			)
		}
		b.WriteString("\n")
	}

	if len(files) == 0 {
		_, err := io.WriteString(writer, b.String())
		return err
	}

	b.WriteString("<details>\n<summary>Uncovered details</summary>\n\n")
	for _, file := range files {
		fmt.Fprintf(&b, "#### %s\n\n", file)
		uncoveredLines := report.UncoveredLines(file)
		if len(uncoveredLines) == 0 {
			b.WriteString("**Uncovered lines:** none\n\n")
		} else {
			parts := make([]string, 0, len(uncoveredLines))
			for _, line := range uncoveredLines {
				parts = append(parts, fmt.Sprintf("%d", line))
			}
			fmt.Fprintf(&b, "**Uncovered lines:** %s\n\n", strings.Join(parts, ", "))
		}

		uncoveredBranches := report.UncoveredBranches(file)
		if len(uncoveredBranches) == 0 {
			b.WriteString("**Uncovered branches:** none\n\n")
		} else {
			fmt.Fprintf(&b, "**Uncovered branches:** %s\n\n", strings.Join(uncoveredBranches, ", "))
		}
	}
	b.WriteString("</details>\n")

	_, err := io.WriteString(writer, b.String())
	return err
}

func markdownFiles(report coverage.Report, includeTplSources bool) []string {
	files := sortedFiles(report)
	if includeTplSources {
		return files
	}
	filtered := make([]string, 0, len(files))
	for _, file := range files {
		if coverage.IsTplSource(file) {
			continue
		}
		filtered = append(filtered, file)
	}
	return filtered
}

func thresholdStatus(rate float64, threshold float64) string {
	if rate >= threshold {
		return "(met)"
	}
	return "(not met)"
}
