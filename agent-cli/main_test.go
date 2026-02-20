package main

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

const testBinaryPath = "bin/agent-cli"

func TestMain(m *testing.M) {
	// Build the binary once before running tests.
	build := exec.Command("go", "build", "-o", testBinaryPath, ".")
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		panic("failed to build test binary: " + err.Error())
	}
	os.Exit(m.Run())
}

func TestMainDispatch_Version(t *testing.T) {
	cmd := exec.Command("./"+testBinaryPath, "version")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("expected exit code 0, got error: %v", err)
	}

	got := string(out)
	if !strings.Contains(got, "agent-cli version ") {
		t.Errorf("stdout = %q, want substring %q", got, "agent-cli version ")
	}
}

func TestMainDispatch_VersionRejectsArgs(t *testing.T) {
	cmd := exec.Command("./"+testBinaryPath, "version", "foo")
	var stderr strings.Builder
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err == nil {
		t.Fatal("expected non-zero exit code, got 0")
	}

	got := stderr.String()
	if !strings.Contains(got, "does not accept positional arguments") {
		t.Errorf("stderr = %q, want substring %q", got, "does not accept positional arguments")
	}
}
