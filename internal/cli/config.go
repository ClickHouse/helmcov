package cli

import (
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

type Config struct {
	ChartPath          string
	TestsPath          string
	ChartsRootPath     string
	Verbose            bool
	Formats            []string
	Threshold          float64
	MaxScenarios       int
	Seed               int64
	GoCoverProfilePath string
	CoberturaPath      string
}

func ParseConfig(args []string) (Config, error) {
	var cfg Config

	fs := flag.NewFlagSet("helmcov", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	var formats multiValue
	fs.Var(&formats, "format", "output format (go|cobertura)")
	fs.StringVar(&cfg.ChartPath, "chart", "", "path to Helm chart")
	fs.StringVar(&cfg.TestsPath, "tests", "", "path to helm-unittest suites")
	fs.StringVar(&cfg.ChartsRootPath, "charts", "", "root path containing nested Helm charts")
	fs.StringVar(&cfg.ChartsRootPath, "charts-root", "", "deprecated alias for --charts")
	fs.Float64Var(&cfg.Threshold, "threshold", 0, "minimum coverage threshold")
	fs.IntVar(&cfg.MaxScenarios, "max-scenarios", 20, "max generated scenarios")
	fs.Int64Var(&cfg.Seed, "seed", 42, "seed for generated scenarios")
	fs.StringVar(&cfg.GoCoverProfilePath, "go-coverprofile", "coverage.out", "path for go coverprofile output")
	fs.StringVar(&cfg.CoberturaPath, "cobertura-file", "coverage.xml", "path for cobertura XML output")
	fs.BoolVar(&cfg.Verbose, "verbose", false, "print per-file and uncovered coverage details")

	if err := fs.Parse(args); err != nil {
		return Config{}, err
	}

	if cfg.ChartPath != "" && cfg.ChartsRootPath != "" {
		return Config{}, errors.New("--chart and --charts are mutually exclusive")
	}
	if cfg.ChartPath == "" && cfg.ChartsRootPath == "" {
		return Config{}, errors.New("either --chart or --charts is required")
	}
	if cfg.ChartPath != "" && cfg.TestsPath == "" {
		cfg.TestsPath = filepath.Join(cfg.ChartPath, "tests")
	}
	if cfg.ChartsRootPath != "" && cfg.TestsPath != "" {
		return Config{}, errors.New("--tests is not supported with --charts; use per-chart tests directories")
	}

	if len(formats) == 0 {
		formats = []string{"go", "cobertura"}
	}
	cfg.Formats = formats

	if cfg.ChartPath != "" {
		if err := validateChartPath(cfg.ChartPath); err != nil {
			return Config{}, err
		}
		if err := validateTestsPath(cfg.TestsPath); err != nil {
			return Config{}, err
		}
	}
	if cfg.ChartsRootPath != "" {
		if err := validateChartsRootPath(cfg.ChartsRootPath); err != nil {
			return Config{}, err
		}
	}

	if cfg.Threshold < 0 || cfg.Threshold > 100 {
		return Config{}, fmt.Errorf("threshold must be between 0 and 100, got %v", cfg.Threshold)
	}
	if cfg.MaxScenarios <= 0 {
		return Config{}, fmt.Errorf("max-scenarios must be > 0, got %d", cfg.MaxScenarios)
	}
	if cfg.GoCoverProfilePath == "" {
		return Config{}, errors.New("--go-coverprofile must not be empty")
	}
	if cfg.CoberturaPath == "" {
		return Config{}, errors.New("--cobertura-file must not be empty")
	}

	for _, format := range cfg.Formats {
		switch format {
		case "go", "cobertura":
		default:
			return Config{}, fmt.Errorf("unsupported format %q", format)
		}
	}

	return cfg, nil
}

func validateChartPath(chartPath string) error {
	info, err := os.Stat(chartPath)
	if err != nil {
		return fmt.Errorf("stat chart path: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("chart path must be a directory: %s", chartPath)
	}

	chartFile := filepath.Join(chartPath, "Chart.yaml")
	if _, err := os.Stat(chartFile); err != nil {
		return fmt.Errorf("chart path must contain Chart.yaml: %w", err)
	}

	return nil
}

func validateTestsPath(testsPath string) error {
	info, err := os.Stat(testsPath)
	if err != nil {
		return fmt.Errorf("stat tests path: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("tests path must be a directory: %s", testsPath)
	}

	hasSuites := false
	err = filepath.WalkDir(testsPath, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		matched, matchErr := filepath.Match("*_test.yaml", filepath.Base(path))
		if matchErr != nil {
			return matchErr
		}
		if matched {
			hasSuites = true
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("scan tests path: %w", err)
	}

	if !hasSuites {
		return fmt.Errorf("tests path must contain at least one *_test.yaml file: %s", testsPath)
	}

	return nil
}

func validateChartsRootPath(root string) error {
	info, err := os.Stat(root)
	if err != nil {
		return fmt.Errorf("stat charts root path: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("charts root path must be a directory: %s", root)
	}
	foundChart := false
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if filepath.Base(path) == "Chart.yaml" {
			foundChart = true
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("scan charts root path: %w", err)
	}
	if !foundChart {
		return fmt.Errorf("charts root path must contain at least one Chart.yaml: %s", root)
	}
	return nil
}

type multiValue []string

func (m *multiValue) String() string {
	return fmt.Sprintf("%v", []string(*m))
}

func (m *multiValue) Set(value string) error {
	*m = append(*m, value)
	return nil
}
