package reporters

import (
	"encoding/xml"
	"fmt"
	"io"
	"sort"
	"strconv"

	"github.com/jordan-simonovski/helmcov/internal/coverage"
)

func WriteGoCoverProfile(report coverage.Report, writer io.Writer) error {
	if _, err := io.WriteString(writer, "mode: count\n"); err != nil {
		return err
	}

	files := sortedFiles(report)
	for _, file := range files {
		lines := sortedLines(report.Files[file].Lines)
		for _, line := range lines {
			hits := report.Files[file].Lines[line]
			if _, err := fmt.Fprintf(writer, "%s:%d.1,%d.1 1 %d\n", file, line, line, hits); err != nil {
				return err
			}
		}
	}
	return nil
}

func WriteCoberturaXML(report coverage.Report, writer io.Writer) error {
	files := sortedFiles(report)
	coverageRoot := coberturaCoverage{
		LineRate:   fmt.Sprintf("%.4f", report.LineRate()),
		BranchRate: fmt.Sprintf("%.4f", report.BranchRate()),
		Version:    "helmcov",
		Sources: []coberturaSource{
			{Source: "."},
		},
		Packages: make([]coberturaPackage, 0, len(files)),
	}

	for _, file := range files {
		fc := report.Files[file]
		lines := sortedLines(fc.Lines)
		class := coberturaClass{
			Name:     file,
			Filename: file,
			Lines:    make([]coberturaLine, 0, len(lines)),
		}
		for _, line := range lines {
			class.Lines = append(class.Lines, coberturaLine{
				Number: strconv.Itoa(line),
				Hits:   strconv.Itoa(fc.Lines[line]),
			})
		}
		coverageRoot.Packages = append(coverageRoot.Packages, coberturaPackage{
			Name: "templates",
			Classes: coberturaClasses{
				Classes: []coberturaClass{class},
			},
		})
	}

	enc := xml.NewEncoder(writer)
	enc.Indent("", "  ")
	if _, err := io.WriteString(writer, xml.Header); err != nil {
		return err
	}
	return enc.Encode(coverageRoot)
}

func sortedFiles(report coverage.Report) []string {
	files := make([]string, 0, len(report.Files))
	for file := range report.Files {
		files = append(files, file)
	}
	sort.Strings(files)
	return files
}

func sortedLines(lines map[int]int) []int {
	keys := make([]int, 0, len(lines))
	for line := range lines {
		keys = append(keys, line)
	}
	sort.Ints(keys)
	return keys
}

type coberturaCoverage struct {
	XMLName    xml.Name           `xml:"coverage"`
	LineRate   string             `xml:"line-rate,attr"`
	BranchRate string             `xml:"branch-rate,attr"`
	Version    string             `xml:"version,attr"`
	Sources    []coberturaSource  `xml:"sources>source"`
	Packages   []coberturaPackage `xml:"packages>package"`
}

type coberturaSource struct {
	Source string `xml:",chardata"`
}

type coberturaPackage struct {
	Name    string           `xml:"name,attr"`
	Classes coberturaClasses `xml:"classes"`
}

type coberturaClasses struct {
	Classes []coberturaClass `xml:"class"`
}

type coberturaClass struct {
	Name     string          `xml:"name,attr"`
	Filename string          `xml:"filename,attr"`
	Lines    []coberturaLine `xml:"lines>line"`
}

type coberturaLine struct {
	Number string `xml:"number,attr"`
	Hits   string `xml:"hits,attr"`
}
