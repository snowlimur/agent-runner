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

func TestSaveRunArtifactsSplitsJSONObjectsAndOtherLines(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "runs")
	runDir := filepath.Join(dir, "20260215T101112-run-1")

	stdout := `{"type":"system","message":"ready"}
stdout line
{"broken":
[1,2,3]
{}
`
	stderr := `{"type":"result","ok":true}
"scalar"
`

	err := SaveRunArtifacts(runDir, stdout, stderr)
	if err != nil {
		t.Fatalf("save artifacts: %v", err)
	}

	ndjsonPath := filepath.Join(runDir, outputNDJSONFileName)
	outputPath := filepath.Join(runDir, outputFileName)

	ndjsonContent, err := os.ReadFile(ndjsonPath)
	if err != nil {
		t.Fatalf("read ndjson log: %v", err)
	}
	expectedNDJSON := `{"type":"system","message":"ready"}
{}
{"type":"result","ok":true}
`
	if got := string(ndjsonContent); got != expectedNDJSON {
		t.Fatalf("unexpected ndjson content: %q", got)
	}

	outputContent, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read output log: %v", err)
	}
	expectedOutput := `stdout line
{"broken":
[1,2,3]
"scalar"
`
	if got := string(outputContent); got != expectedOutput {
		t.Fatalf("unexpected output content: %q", got)
	}
}

func TestSaveRunArtifactsCreatesBothFilesWhenOneIsEmpty(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name       string
		stdout     string
		stderr     string
		wantNDJSON string
		wantOutput string
	}{
		{
			name:       "json_only",
			stdout:     `{"type":"result","ok":true}` + "\n",
			stderr:     "",
			wantNDJSON: `{"type":"result","ok":true}` + "\n",
			wantOutput: "",
		},
		{
			name:       "plain_only",
			stdout:     "plain line\n",
			stderr:     "",
			wantNDJSON: "",
			wantOutput: "plain line\n",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			dir := filepath.Join(t.TempDir(), "runs")
			runDir := filepath.Join(dir, "20260215T101112-run-1")

			err := SaveRunArtifacts(runDir, tc.stdout, tc.stderr)
			if err != nil {
				t.Fatalf("save artifacts: %v", err)
			}

			ndjsonPath := filepath.Join(runDir, outputNDJSONFileName)
			outputPath := filepath.Join(runDir, outputFileName)

			if _, err := os.Stat(ndjsonPath); err != nil {
				t.Fatalf("stat ndjson log: %v", err)
			}
			if _, err := os.Stat(outputPath); err != nil {
				t.Fatalf("stat output log: %v", err)
			}

			ndjsonContent, err := os.ReadFile(ndjsonPath)
			if err != nil {
				t.Fatalf("read ndjson log: %v", err)
			}
			if got := string(ndjsonContent); got != tc.wantNDJSON {
				t.Fatalf("unexpected ndjson content: %q", got)
			}

			outputContent, err := os.ReadFile(outputPath)
			if err != nil {
				t.Fatalf("read output log: %v", err)
			}
			if got := string(outputContent); got != tc.wantOutput {
				t.Fatalf("unexpected output content: %q", got)
			}
		})
	}
}
