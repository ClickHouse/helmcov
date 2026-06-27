package chartloader

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadChartFilesLoadsNestedFiles(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	filesDir := filepath.Join(root, "files", "configs")
	if err := os.MkdirAll(filesDir, 0o755); err != nil {
		t.Fatalf("mkdir files: %v", err)
	}
	writeFile(t, filepath.Join(filesDir, "app.yaml"), "enabled: true\n")
	writeFile(t, filepath.Join(root, "files", "notes.txt"), "note\n")

	files, err := LoadChartFiles(root)
	if err != nil {
		t.Fatalf("load chart files: %v", err)
	}

	if content, err := files.Get("notes.txt"); err != nil || content != "note\n" {
		t.Fatalf("expected notes.txt content, got %q err=%v", content, err)
	}
	if content, err := files.Get("configs/app.yaml"); err != nil || content == "" {
		t.Fatalf("expected nested file content, got %q err=%v", content, err)
	}
}

func TestChartFilesGlobSupportsStarAndDoubleStar(t *testing.T) {
	t.Parallel()

	files := ChartFiles{files: map[string][]byte{
		"configs/app.yaml": []byte("a"),
		"configs/db.yaml":  []byte("b"),
		"notes.txt":        []byte("c"),
	}}

	matches, err := files.Glob("configs/*.yaml")
	if err != nil {
		t.Fatalf("glob configs/*.yaml: %v", err)
	}
	if len(matches) != 2 {
		t.Fatalf("expected 2 yaml matches, got %v", matches)
	}

	matches, err = files.Glob("configs/**")
	if err != nil {
		t.Fatalf("glob configs/**: %v", err)
	}
	if len(matches) != 2 {
		t.Fatalf("expected 2 recursive matches, got %v", matches)
	}
}

func TestChartFilesLinesAndAsConfig(t *testing.T) {
	t.Parallel()

	files := ChartFiles{files: map[string][]byte{
		"app.yaml": []byte("enabled: true\nmode: prod\n"),
	}}

	lines, err := files.Lines("app.yaml")
	if err != nil {
		t.Fatalf("lines: %v", err)
	}
	if len(lines) < 2 {
		t.Fatalf("expected at least 2 lines, got %v", lines)
	}

	config, err := files.AsConfig()
	if err != nil {
		t.Fatalf("asConfig: %v", err)
	}
	if config == "" {
		t.Fatalf("expected config bundle output")
	}
}
