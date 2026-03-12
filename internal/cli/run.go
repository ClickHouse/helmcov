package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/jordan-simonovski/helmcov/internal/chartloader"
	"github.com/jordan-simonovski/helmcov/internal/coverage"
	"github.com/jordan-simonovski/helmcov/internal/instrumentation"
	"github.com/jordan-simonovski/helmcov/internal/reporters"
	"github.com/jordan-simonovski/helmcov/internal/valuegen"
)

func Run(args []string, stdout io.Writer) error {
	cfg, err := ParseConfig(args)
	if err != nil {
		return err
	}

	targets, err := buildTargets(cfg)
	if err != nil {
		return err
	}

	exec := instrumentation.NewExecutor()
	traces := make([]instrumentation.Trace, 0, 32)
	totalSuites := 0
	for _, target := range targets {
		bundle, loadErr := chartloader.LoadBundle(target.ChartPath, target.TestsPath)
		if loadErr != nil {
			return loadErr
		}
		totalSuites += len(bundle.Suites)
		for _, suite := range bundle.Suites {
			scenarios := valuegen.Generate(suite.MergedValues, valuegen.Options{
				MaxScenarios: cfg.MaxScenarios,
				Seed:         cfg.Seed,
			})
			for _, templatePath := range suite.Templates {
				templateFile := templatePath
				if !filepath.IsAbs(templateFile) {
					templateFile = filepath.Join(target.ChartPath, templatePath)
				}
				content, readErr := os.ReadFile(templateFile)
				if readErr != nil {
					return fmt.Errorf("read template %s: %w", templateFile, readErr)
				}
				for _, scenario := range scenarios {
					trace, _, traceErr := exec.RenderAndTrace(templateFile, string(content), map[string]any{
						"Values": scenario,
					})
					if traceErr != nil {
						return fmt.Errorf("render template %s: %w", templatePath, traceErr)
					}
					traces = append(traces, trace)
				}
			}
		}
	}

	report := coverage.FromTraces(traces)
	if err := writeRequestedFormats(cfg, report); err != nil {
		return err
	}

	lineCoverage := report.LineRate() * 100
	if cfg.Threshold > 0 && lineCoverage < cfg.Threshold {
		return fmt.Errorf("coverage %.2f is below threshold %.2f", lineCoverage, cfg.Threshold)
	}

	formats := append([]string(nil), cfg.Formats...)
	sort.Strings(formats)
	_, err = fmt.Fprintf(
		stdout,
		"helmcov: target=%s tests=%s formats=%s suites=%d line-coverage=%.2f%% branch-coverage=%.2f%%\n",
		targetLabel(cfg),
		testsLabel(cfg),
		strings.Join(formats, ","),
		totalSuites,
		lineCoverage,
		report.BranchRate()*100,
	)
	if err != nil {
		return err
	}
	if cfg.Verbose {
		if err := writeVerboseCoverage(stdout, report); err != nil {
			return err
		}
	}
	return nil
}

type target struct {
	ChartPath string
	TestsPath string
}

func buildTargets(cfg Config) ([]target, error) {
	if cfg.ChartsRootPath == "" {
		return []target{{ChartPath: cfg.ChartPath, TestsPath: cfg.TestsPath}}, nil
	}
	chartPaths, err := chartloader.DiscoverCharts(cfg.ChartsRootPath)
	if err != nil {
		return nil, err
	}
	targets := make([]target, 0, len(chartPaths))
	for _, chartPath := range chartPaths {
		testsPath := filepath.Join(chartPath, "tests")
		if err := validateTestsPath(testsPath); err != nil {
			continue
		}
		targets = append(targets, target{ChartPath: chartPath, TestsPath: testsPath})
	}
	if len(targets) == 0 {
		return nil, fmt.Errorf("no chart/test targets found under charts root %s", cfg.ChartsRootPath)
	}
	return targets, nil
}

func targetLabel(cfg Config) string {
	if cfg.ChartsRootPath != "" {
		return cfg.ChartsRootPath
	}
	return cfg.ChartPath
}

func testsLabel(cfg Config) string {
	if cfg.ChartsRootPath != "" {
		return "<auto: chart/tests>"
	}
	return cfg.TestsPath
}

func writeRequestedFormats(cfg Config, report coverage.Report) error {
	for _, format := range cfg.Formats {
		switch format {
		case "go":
			if err := writeToFile(cfg.GoCoverProfilePath, func(w io.Writer) error {
				return reporters.WriteGoCoverProfile(report, w)
			}); err != nil {
				return fmt.Errorf("write go coverprofile: %w", err)
			}
		case "cobertura":
			if err := writeToFile(cfg.CoberturaPath, func(w io.Writer) error {
				return reporters.WriteCoberturaXML(report, w)
			}); err != nil {
				return fmt.Errorf("write cobertura XML: %w", err)
			}
		default:
			return fmt.Errorf("unsupported format %q", format)
		}
	}
	return nil
}

func writeToFile(path string, writeFn func(io.Writer) error) (err error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() {
		closeErr := file.Close()
		if err == nil && closeErr != nil {
			err = closeErr
		}
	}()

	err = writeFn(file)
	return err
}

func writeVerboseCoverage(stdout io.Writer, report coverage.Report) error {
	if _, err := io.WriteString(stdout, "coverage details:\n"); err != nil {
		return err
	}
	for _, file := range report.SortedFiles() {
		covered, total := report.FileCoveredLineCount(file)
		uncoveredLines := report.UncoveredLines(file)
		uncoveredBranches := report.UncoveredBranches(file)
		if _, err := fmt.Fprintf(
			stdout,
			"template: %s\n  line-coverage: %.2f%% (%d/%d)\n",
			file,
			report.FileLineRate(file)*100,
			covered,
			total,
		); err != nil {
			return err
		}
		if _, err := io.WriteString(stdout, "  uncovered lines:\n"); err != nil {
			return err
		}
		if len(uncoveredLines) == 0 {
			if _, err := io.WriteString(stdout, "    - none\n"); err != nil {
				return err
			}
		} else {
			for _, ref := range uncoveredLineRefs(file, uncoveredLines) {
				if _, err := fmt.Fprintf(stdout, "    - %s\n", ref); err != nil {
					return err
				}
			}
		}
		if _, err := io.WriteString(stdout, "  uncovered branches:\n"); err != nil {
			return err
		}
		if len(uncoveredBranches) == 0 {
			if _, err := io.WriteString(stdout, "    - none\n"); err != nil {
				return err
			}
		} else {
			for _, ref := range uncoveredBranchRefs(file, uncoveredBranches) {
				if _, err := fmt.Fprintf(stdout, "    - %s\n", ref); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func uncoveredLineRefs(file string, uncoveredLines []int) []string {
	if len(uncoveredLines) == 0 {
		return nil
	}
	content, err := os.ReadFile(file)
	if err != nil {
		refs := make([]string, 0, len(uncoveredLines))
		for _, line := range uncoveredLines {
			refs = append(refs, fmt.Sprintf("%s:%d", file, line))
		}
		return refs
	}

	lines := strings.Split(string(content), "\n")
	refs := make([]string, 0, len(uncoveredLines))
	for _, lineNumber := range uncoveredLines {
		source := ""
		if lineNumber-1 >= 0 && lineNumber-1 < len(lines) {
			source = strings.TrimSpace(lines[lineNumber-1])
		}
		refs = append(refs, fmt.Sprintf("%s:%d %q", file, lineNumber, source))
	}
	return refs
}

func uncoveredBranchRefs(file string, uncoveredBranches []string) []string {
	if len(uncoveredBranches) == 0 {
		return nil
	}

	content, err := os.ReadFile(file)
	if err != nil {
		refs := make([]string, 0, len(uncoveredBranches))
		for _, branch := range uncoveredBranches {
			refs = append(refs, fmt.Sprintf("%s (%s)", file, branch))
		}
		return refs
	}
	lines := strings.Split(string(content), "\n")

	refs := make([]string, 0, len(uncoveredBranches))
	for _, branch := range uncoveredBranches {
		line, edge, ok := parseBranchRef(branch)
		if !ok {
			refs = append(refs, fmt.Sprintf("%s (%s)", file, branch))
			continue
		}
		source := ""
		if line-1 >= 0 && line-1 < len(lines) {
			source = strings.TrimSpace(lines[line-1])
		}
		refs = append(refs, fmt.Sprintf("%s:%d %q (%s)", file, line, source, edge))
	}
	return refs
}

func parseBranchRef(branch string) (line int, edge string, ok bool) {
	parts := strings.Split(branch, ":")
	if len(parts) < 3 {
		return 0, "", false
	}
	lineNumber, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, "", false
	}
	return lineNumber, strings.Join(parts[1:], ":"), true
}
