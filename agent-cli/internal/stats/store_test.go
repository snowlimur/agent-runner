package stats

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSaveRunRecordCreatesFile(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "runs")
	record := &RunRecord{
		Timestamp: time.Now().UTC(),
		Status:    RunStatusSuccess,
		CWD:       "/tmp/work",
	}

	path, err := SaveRunRecord(dir, record)
	if err != nil {
		t.Fatalf("save record: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("stat saved file: %v", err)
	}
	if filepath.Base(path) != statsFileName {
		t.Fatalf("expected stats file name %q, got %q", statsFileName, filepath.Base(path))
	}
	if record.RunID == "" {
		t.Fatal("expected generated run id")
	}
	runDir := filepath.Base(filepath.Dir(path))
	if runDir == record.RunID {
		t.Fatalf("run directory must include timestamp prefix, got %q", runDir)
	}
	expectedSuffix := "-" + sanitizeID(record.RunID)
	if !strings.HasSuffix(runDir, expectedSuffix) {
		t.Fatalf("expected run directory suffix %q, got %q", expectedSuffix, runDir)
	}
	prefix := strings.TrimSuffix(runDir, expectedSuffix)
	if prefix == runDir {
		t.Fatalf("expected timestamp prefix in run directory, got %q", runDir)
	}
	if _, err := time.Parse(runDirTimestampFormat, prefix); err != nil {
		t.Fatalf("run directory timestamp %q does not match %q: %v", prefix, runDirTimestampFormat, err)
	}
}

func TestSaveRunRecordUniqueNames(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "runs")
	recordA := &RunRecord{
		Timestamp: time.Now().UTC(),
		Status:    RunStatusSuccess,
	}
	recordB := &RunRecord{
		Timestamp: time.Now().UTC(),
		Status:    RunStatusSuccess,
	}

	pathA, err := SaveRunRecord(dir, recordA)
	if err != nil {
		t.Fatalf("save first: %v", err)
	}
	pathB, err := SaveRunRecord(dir, recordB)
	if err != nil {
		t.Fatalf("save second: %v", err)
	}

	if pathA == pathB {
		t.Fatalf("expected unique paths, got %q", pathA)
	}
	dirA := filepath.Base(filepath.Dir(pathA))
	dirB := filepath.Base(filepath.Dir(pathB))
	if dirA == dirB {
		t.Fatalf("expected unique run directory names, got %q", dirA)
	}
	suffixA := "-" + sanitizeID(recordA.RunID)
	suffixB := "-" + sanitizeID(recordB.RunID)
	if !strings.HasSuffix(dirA, suffixA) {
		t.Fatalf("expected run dir %q to end with %q", dirA, suffixA)
	}
	if !strings.HasSuffix(dirB, suffixB) {
		t.Fatalf("expected run dir %q to end with %q", dirB, suffixB)
	}
}

func TestSaveRunArtifactsCreatesOnlyOutputFile(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "runs")
	runDir := filepath.Join(dir, "20260215T101112-run-1")

	err := SaveRunArtifacts(runDir, "stdout line\n", "stderr line\n")
	if err != nil {
		t.Fatalf("save artifacts: %v", err)
	}

	outputPath := filepath.Join(runDir, outputFileName)

	if _, err := os.Stat(filepath.Join(runDir, "prompt.md")); !os.IsNotExist(err) {
		t.Fatalf("expected prompt.md to be absent, got err=%v", err)
	}

	outputContent, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read output log: %v", err)
	}
	if got := string(outputContent); got != "stdout line\nstderr line\n" {
		t.Fatalf("unexpected output content: %q", got)
	}
}
