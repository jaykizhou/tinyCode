package cli_test

import (
	"bytes"
	"strings"
	"testing"

	"tinycode/internal/cli"
)

func TestVersionCmdOutput(t *testing.T) {
	oldVersion := cli.Version
	oldCommit := cli.Commit
	oldBuildDate := cli.BuildDate
	defer func() {
		cli.Version = oldVersion
		cli.Commit = oldCommit
		cli.BuildDate = oldBuildDate
	}()

	cli.Version = "1.0.0-test"
	cli.Commit = "abc123"
	cli.BuildDate = "2026-04-29"

	root := cli.NewRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"version"})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "1.0.0-test") {
		t.Errorf("output missing Version: %s", output)
	}
	if !strings.Contains(output, "abc123") {
		t.Errorf("output missing Commit: %s", output)
	}
	if !strings.Contains(output, "2026-04-29") {
		t.Errorf("output missing BuildDate: %s", output)
	}
}
