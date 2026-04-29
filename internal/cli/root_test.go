package cli_test

import (
	"testing"

	"tinycode/internal/cli"
)

func TestNewRootCmd(t *testing.T) {
	cmd := cli.NewRootCmd()
	if cmd == nil {
		t.Fatal("expected non-nil *cobra.Command")
	}
}

func TestRootCmdHasVersionSubcommand(t *testing.T) {
	cmd := cli.NewRootCmd()
	for _, c := range cmd.Commands() {
		if c.Name() == "version" {
			return
		}
	}
	t.Error("root command missing 'version' subcommand")
}

func TestRootCmdHasReplSubcommand(t *testing.T) {
	cmd := cli.NewRootCmd()
	for _, c := range cmd.Commands() {
		if c.Name() == "repl" {
			return
		}
	}
	t.Error("root command missing 'repl' subcommand")
}

func TestRootCmdPersistentFlags(t *testing.T) {
	cmd := cli.NewRootCmd()
	flags := []string{"base-url", "model", "api-key", "work-dir", "max-iter", "system", "verbose"}
	for _, name := range flags {
		if cmd.PersistentFlags().Lookup(name) == nil {
			t.Errorf("missing persistent flag: --%s", name)
		}
	}
}
