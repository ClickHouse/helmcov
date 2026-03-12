package coverage

import (
	"sort"
	"strconv"
	"strings"

	"github.com/jordan-simonovski/helmcov/internal/instrumentation"
)

type Report struct {
	Files map[string]FileCoverage
}

type FileCoverage struct {
	Lines    map[int]int
	Branches map[string]int
}

func (r Report) LineRate() float64 {
	total := 0
	covered := 0
	for _, file := range r.Files {
		for _, hits := range file.Lines {
			total++
			if hits > 0 {
				covered++
			}
		}
	}
	if total == 0 {
		return 0
	}
	return float64(covered) / float64(total)
}

func (r Report) BranchRate() float64 {
	total := 0
	covered := 0
	for _, file := range r.Files {
		for _, hits := range file.Branches {
			total++
			if hits > 0 {
				covered++
			}
		}
	}
	if total == 0 {
		return 0
	}
	return float64(covered) / float64(total)
}

func (r Report) SortedFiles() []string {
	files := make([]string, 0, len(r.Files))
	for file := range r.Files {
		files = append(files, file)
	}
	sort.Strings(files)
	return files
}

func (r Report) FileLineRate(file string) float64 {
	fc, ok := r.Files[file]
	if !ok {
		return 0
	}
	total := len(fc.Lines)
	if total == 0 {
		return 0
	}
	covered := 0
	for _, hits := range fc.Lines {
		if hits > 0 {
			covered++
		}
	}
	return float64(covered) / float64(total)
}

func (r Report) FileCoveredLineCount(file string) (covered int, total int) {
	fc, ok := r.Files[file]
	if !ok {
		return 0, 0
	}
	total = len(fc.Lines)
	for _, hits := range fc.Lines {
		if hits > 0 {
			covered++
		}
	}
	return covered, total
}

func (r Report) UncoveredLines(file string) []int {
	fc, ok := r.Files[file]
	if !ok {
		return nil
	}
	lines := make([]int, 0)
	for line, hits := range fc.Lines {
		if hits == 0 {
			lines = append(lines, line)
		}
	}
	sort.Ints(lines)
	return lines
}

func (r Report) UncoveredBranches(file string) []string {
	fc, ok := r.Files[file]
	if !ok {
		return nil
	}
	branches := make([]string, 0)
	for branch, hits := range fc.Branches {
		if hits == 0 {
			branches = append(branches, branch)
		}
	}
	sort.Strings(branches)
	return branches
}

func FromTraces(traces []instrumentation.Trace) Report {
	report := Report{Files: map[string]FileCoverage{}}

	for _, trace := range traces {
		for key, hitCount := range trace.Lines {
			file, line, ok := splitLineKey(key)
			if !ok {
				continue
			}
			fc := report.Files[file]
			if fc.Lines == nil {
				fc.Lines = map[int]int{}
			}
			if fc.Branches == nil {
				fc.Branches = map[string]int{}
			}
			fc.Lines[line] += hitCount
			report.Files[file] = fc
		}
		for key, hitCount := range trace.Branches {
			file, branch, ok := splitBranchKey(key)
			if !ok {
				continue
			}
			fc := report.Files[file]
			if fc.Lines == nil {
				fc.Lines = map[int]int{}
			}
			if fc.Branches == nil {
				fc.Branches = map[string]int{}
			}
			fc.Branches[branch] += hitCount
			report.Files[file] = fc
		}
	}

	return report
}

func splitLineKey(key string) (string, int, bool) {
	idx := strings.LastIndex(key, ":")
	if idx == -1 {
		return "", 0, false
	}
	line, err := strconv.Atoi(key[idx+1:])
	if err != nil {
		return "", 0, false
	}
	return key[:idx], line, true
}

func splitBranchKey(key string) (string, string, bool) {
	last := strings.LastIndex(key, ":")
	if last == -1 {
		return "", "", false
	}
	prev := strings.LastIndex(key[:last], ":")
	if prev == -1 {
		return "", "", false
	}
	beforePrev := strings.LastIndex(key[:prev], ":")
	if beforePrev == -1 {
		// Legacy format: file:if:true
		return key[:prev], key[prev+1:], true
	}
	// Current format: file:line:if:true
	return key[:beforePrev], key[beforePrev+1:], true
}
