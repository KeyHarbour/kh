package cli

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	internalconfig "kh/internal/config"

	"github.com/spf13/cobra"
)

func TestNewAuthCmd_HasExpectedSubcommands(t *testing.T) {
	cmd := newAuthCmd()
	if cmd.Use != "auth" {
		t.Fatalf("expected use auth, got %s", cmd.Use)
	}

	seen := map[string]bool{}
	for _, sub := range cmd.Commands() {
		seen[sub.Name()] = true
	}

	for _, want := range []string{"login", "logout", "whoami"} {
		if !seen[want] {
			t.Fatalf("expected subcommand %s to be present", want)
		}
	}
}

func TestCompletionRunE_GeneratesScripts(t *testing.T) {
	for _, shell := range []string{"bash", "zsh", "fish", "powershell"} {
		t.Run(shell, func(t *testing.T) {
			root := &cobra.Command{Use: "kh"}
			root.Run = func(cmd *cobra.Command, args []string) {}
			completion := newCompletionCmd(root)

			oldStdout := os.Stdout
			readPipe, writePipe, err := os.Pipe()
			if err != nil {
				t.Fatalf("Pipe error: %v", err)
			}
			os.Stdout = writePipe
			defer func() { os.Stdout = oldStdout }()

			if err := completion.RunE(completion, []string{shell}); err != nil {
				_ = writePipe.Close()
				t.Fatalf("RunE error: %v", err)
			}
			_ = writePipe.Close()

			buf := &bytes.Buffer{}
			if _, err := io.Copy(buf, readPipe); err != nil {
				t.Fatalf("Copy error: %v", err)
			}
			_ = readPipe.Close()

			if buf.Len() == 0 {
				t.Fatal("expected completion output")
			}
			if !strings.Contains(buf.String(), "kh") {
				t.Fatalf("expected script to mention command name, got %q", buf.String())
			}
		})
	}
}

func TestCompletionRunE_UnknownShellIsNoOp(t *testing.T) {
	root := &cobra.Command{Use: "kh"}
	completion := newCompletionCmd(root)
	if err := completion.RunE(completion, []string{"unknown"}); err != nil {
		t.Fatalf("expected nil error for unmatched shell branch, got %v", err)
	}
}

func TestConfigCommand_HasExpectedSubcommands(t *testing.T) {
	cmd := newConfigCmd()
	if cmd.Use != "config" {
		t.Fatalf("expected use config, got %s", cmd.Use)
	}

	seen := map[string]bool{}
	for _, sub := range cmd.Commands() {
		seen[sub.Name()] = true
	}

	for _, want := range []string{"get", "set"} {
		if !seen[want] {
			t.Fatalf("expected subcommand %s to be present", want)
		}
	}
}

func TestConfigGetArgsHelpfulErrors(t *testing.T) {
	cmd := newConfigGetCmd()
	if err := cmd.Args(cmd, nil); err == nil || !strings.Contains(err.Error(), "requires 1 argument") {
		t.Fatalf("expected missing arg error, got %v", err)
	}
}

func TestConfigSetArgsHelpfulErrors(t *testing.T) {
	cmd := newConfigSetCmd()
	if err := cmd.Args(cmd, []string{"endpoint"}); err == nil || !strings.Contains(err.Error(), "requires 2 arguments") {
		t.Fatalf("expected missing arg error, got %v", err)
	}
}

func TestConfigSetAndGetCommands(t *testing.T) {
	useTempConfigHome(t)

	setOut := &bytes.Buffer{}
	setCmd := newConfigCmd()
	setCmd.SetOut(setOut)
	setCmd.SetErr(io.Discard)
	setCmd.SetArgs([]string{"set", "endpoint", "https://example.test"})
	if err := setCmd.Execute(); err != nil {
		t.Fatalf("set command failed: %v", err)
	}

	cfg, err := internalconfig.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.Endpoint != "https://example.test" {
		t.Fatalf("expected endpoint to be persisted, got %q", cfg.Endpoint)
	}

	getOut := &bytes.Buffer{}
	getCmd := newConfigCmd()
	getCmd.SetOut(getOut)
	getCmd.SetErr(io.Discard)
	getCmd.SetArgs([]string{"get", "endpoint"})
	if err := getCmd.Execute(); err != nil {
		t.Fatalf("get command failed: %v", err)
	}
	if strings.TrimSpace(getOut.String()) != "https://example.test" {
		t.Fatalf("unexpected get output %q", getOut.String())
	}
}

func TestConfigGetCommand_ReportsMissingToken(t *testing.T) {
	useTempConfigHome(t)

	cmd := newConfigCmd()
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"get", "token"})

	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "no token configured") {
		t.Fatalf("expected missing token error, got %v", err)
	}
}

func TestConfigSetCommand_ReportsInvalidKey(t *testing.T) {
	useTempConfigHome(t)

	cmd := newConfigCmd()
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"set", "unknown", "value"})

	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "unknown key") {
		t.Fatalf("expected unknown key error, got %v", err)
	}
}

func useTempConfigHome(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("KH_ENDPOINT", "")
	t.Setenv("KH_TOKEN", "")
	t.Setenv("KH_ORG", "")
	t.Setenv("KH_PROJECT", "")
	t.Setenv("KH_CONCURRENCY", "")
	t.Setenv("KH_INSECURE", "")
}
