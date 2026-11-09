package cli

import (
	"cmp"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/ClickHouse/helmcov/internal/chartloader"
	"github.com/ClickHouse/helmcov/internal/coverage"
	"github.com/ClickHouse/helmcov/internal/instrumentation"
	"github.com/ClickHouse/helmcov/internal/reporters"
	"github.com/ClickHouse/helmcov/internal/valuegen"
)

// Version is the helmcov version. main sets it once at startup from build
// metadata, the one package variable the no-mutable-state rule allows.
var Version = "dev"

func Run(args []string, stdout io.Writer) error {
	cfg, err := ParseConfig(args)
	if err != nil {
		return err
	}
	if cfg.ShowVersion {
		_, err := fmt.Fprintf(stdout, "helmcov %s\n", Version)
		return err
	}

	targets, err := buildTargets(cfg)
	if err != nil {
		return err
	}

	traces := make([]instrumentation.Trace, 0, 32)
	var testTraces []instrumentation.Trace
	totalSuites := 0
	for _, target := range targets {
		bundle, loadErr := chartloader.LoadBundle(target.ChartPath, target.TestsPath)
		if loadErr != nil {
			return loadErr
		}
		chartTemplates, templatesErr := chartloader.LoadTemplateFiles(target.ChartPath)
		if templatesErr != nil {
			return templatesErr
		}
		chartFiles, filesErr := chartloader.LoadChartFiles(target.ChartPath)
		if filesErr != nil {
			return filesErr
		}
		totalSuites += len(bundle.Suites)
		chart := chartRenderer{
			chartPath: target.ChartPath, meta: bundle.Chart, kubeVersion: cfg.KubeVersion,
			files: chartFiles, templates: chartTemplates, sources: map[string]templateSource{},
		}
		for _, suite := range bundle.Suites {
			for _, job := range suiteRenders(suite, cfg) {
				trace, ok, err := chart.trace(job)
				if err != nil {
					return err
				}
				if ok {
					traces = append(traces, trace)
				}
				if ok && job.test {
					testTraces = append(testTraces, trace)
				} else if ok {
					testTraces = append(testTraces, withoutHits(trace))
				}
			}
		}
	}

	report := coverage.FromTraces(traces)
	testReport := coverage.FromTraces(testTraces)
	if err := writeRequestedFormats(cfg, report, testReport); err != nil {
		return err
	}

	lineCoverage := report.LineRate() * 100
	branchCoverage := report.BranchRate() * 100
	// Check both gates so one run reports every miss.
	var gateErrs []error
	if lineCoverage < cfg.Threshold {
		gateErrs = append(gateErrs, fmt.Errorf("coverage %.2f is below threshold %.2f", lineCoverage, cfg.Threshold))
	}
	if branchCoverage < cfg.BranchThreshold {
		gateErrs = append(gateErrs, fmt.Errorf("branch coverage %.2f is below branch threshold %.2f", branchCoverage, cfg.BranchThreshold))
	}
	if err := errors.Join(gateErrs...); err != nil {
		return err
	}

	formats := append([]string(nil), cfg.Formats...)
	sort.Strings(formats)
	_, err = fmt.Fprintf(
		stdout,
		// The tests-only names must not end in "line-coverage=", which the action's sed matches greedily.
		"helmcov: target=%s tests=%s formats=%s suites=%d line-coverage=%.2f%% branch-coverage=%.2f%% line-coverage-tests=%.2f%% branch-coverage-tests=%.2f%%\n",
		targetLabel(cfg),
		testsLabel(cfg),
		strings.Join(formats, ","),
		totalSuites,
		lineCoverage,
		branchCoverage,
		testReport.LineRate()*100,
		testReport.BranchRate()*100,
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

// withoutHits keeps a trace's keys at zero, so what only scenarios render still
// counts against the tests-only coverage.
func withoutHits(trace instrumentation.Trace) instrumentation.Trace {
	zero := instrumentation.Trace{Lines: map[string]int{}, Branches: map[string]int{}}
	for key := range trace.Lines {
		zero.Lines[key] = 0
	}
	for key := range trace.Branches {
		zero.Branches[key] = 0
	}
	return zero
}

// helmUnittestKubeVersion is the Kubernetes version helm-unittest renders with by default.
const helmUnittestKubeVersion = "1.20.0"

type renderJob struct {
	template string
	values   map[string]any
	context  chartloader.UnittestContext
	// test is true for a helm-unittest test's own values, which render as Helm would.
	test       bool
	expectFail bool
}

// suiteRenders renders the suite's templates with its values and the generated
// scenarios, then each test's templates with that test's own values.
func suiteRenders(suite chartloader.Suite, cfg Config) []renderJob {
	scenarios := valuegen.Generate(suite.MergedValues, valuegen.Options{MaxScenarios: cfg.MaxScenarios, Seed: cfg.Seed})
	var jobs []renderJob
	for _, template := range suite.Templates {
		for _, values := range scenarios {
			jobs = append(jobs, renderJob{template: template, values: values, context: suite.Context()})
		}
	}
	for _, test := range suite.Tests {
		for _, template := range test.TemplatesIn(suite) {
			jobs = append(jobs, renderJob{template: template, values: test.MergedValues, context: test.ContextIn(suite), test: true, expectFail: test.ExpectsRenderFailure()})
		}
	}
	return jobs
}

type templateSource struct {
	file, content string
	skip          bool
}

type chartRenderer struct {
	chartPath   string
	meta        chartloader.ChartMeta
	kubeVersion string
	files       chartloader.ChartFiles
	templates   map[string]string
	sources     map[string]templateSource
}

// trace renders one job. ok is false for templates helmcov skips.
func (c chartRenderer) trace(job renderJob) (instrumentation.Trace, bool, error) {
	templatePath := job.template
	src, err := c.source(templatePath)
	if err != nil || src.skip {
		return instrumentation.Trace{}, false, err
	}
	meta := c.meta
	meta.Version = cmp.Or(job.context.ChartVersion, meta.Version)
	meta.AppVersion = cmp.Or(job.context.ChartAppVersion, meta.AppVersion)
	exec := instrumentation.Executor{Lookup: job.context.Lookup, Strict: job.test}
	trace, _, err := exec.RenderAndTrace(src.file, src.content, chartloader.HelmRenderValues(chartloader.RenderOptions{
		Chart:        meta,
		Values:       job.values,
		ChartPath:    c.chartPath,
		TemplatePath: src.file,
		KubeVersion:  cmp.Or(job.context.KubeVersion, c.kubeVersion, helmUnittestKubeVersion),
		Files:        c.files,
		Release:      job.context.Release,
		APIVersions:  job.context.APIVersions,
	}), c.templates)
	if err != nil {
		// A test that asserts failedTemplate expects this; the lines run before the failure still count.
		if job.expectFail && errors.Is(err, instrumentation.ErrExecute) {
			return trace, true, nil
		}
		return instrumentation.Trace{}, false, fmt.Errorf("render template %s: %w", templatePath, err)
	}
	return trace, true, nil
}

func (c chartRenderer) source(templatePath string) (templateSource, error) {
	if src, ok := c.sources[templatePath]; ok {
		return src, nil
	}
	file, err := chartloader.ResolveTemplatePath(c.chartPath, templatePath)
	if err != nil {
		// A packaged dependency's templates sit inside its .tgz and are not this chart's source.
		if chartloader.IsPackagedSubchartTemplate(c.chartPath, templatePath) {
			c.sources[templatePath] = templateSource{skip: true}
			return c.sources[templatePath], nil
		}
		return templateSource{}, fmt.Errorf("resolve template %s: %w", templatePath, err)
	}
	content, err := os.ReadFile(file)
	if err != nil {
		return templateSource{}, fmt.Errorf("read template %s: %w", file, err)
	}
	c.sources[templatePath] = templateSource{file: file, content: string(content)}
	return c.sources[templatePath], nil
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

func writeRequestedFormats(cfg Config, report, testReport coverage.Report) error {
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
		case "markdown":
			if err := writeToFile(cfg.MarkdownPath, func(w io.Writer) error {
				return reporters.WriteMarkdown(report, w, reporters.MarkdownOptions{
					Threshold:         cfg.Threshold,
					BranchThreshold:   cfg.BranchThreshold,
					CommentMarker:     cfg.CommentMarker,
					ShowFileSummary:   cfg.MarkdownFileSummary,
					IncludeTplSources: cfg.MarkdownIncludeTpl,
					Tests:             &testReport,
				})
			}); err != nil {
				return fmt.Errorf("write markdown: %w", err)
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
	defer func() { err = errors.Join(err, file.Close()) }()
	return writeFn(file)
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
		refs = append(refs, fmt.Sprintf("%s:%d %q", file, lineNumber, sourceLine(lines, lineNumber)))
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
		refs = append(refs, fmt.Sprintf("%s:%d %q (%s)", file, line, sourceLine(lines, line), edge))
	}
	return refs
}

// sourceLine returns the trimmed text of a 1-based line, or "" when it is out of range.
func sourceLine(lines []string, number int) string {
	if number < 1 || number > len(lines) {
		return ""
	}
	return strings.TrimSpace(lines[number-1])
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
