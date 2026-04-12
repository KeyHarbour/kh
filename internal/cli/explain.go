package cli

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/spf13/cobra"
)

// ExplainInfo describes what a command will do before it executes.
// It is produced deterministically from the command's flag values —
// no LLM or network call is required.
type ExplainInfo struct {
	Command     string        `json:"command"`
	Description string        `json:"description"`
	Reads       []string      `json:"reads,omitempty"`
	Writes      []string      `json:"writes,omitempty"`
	SideEffects []string      `json:"side_effects,omitempty"`
	Permissions []string      `json:"permissions"`
	ExitCodes   []ExplainExit `json:"exit_codes"`
}

// ExplainExit maps a numeric exit code to a human description.
type ExplainExit struct {
	Code        string `json:"code"`
	Description string `json:"description"`
}

// argsWithExplain wraps a PositionalArgs validator so it is skipped when
// --explain is passed. Use this on commands that require positional args but
// should work with zero args when --explain is set.
func argsWithExplain(explainFlag *bool, inner cobra.PositionalArgs) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if *explainFlag {
			return nil
		}
		return inner(cmd, args)
	}
}

// printExplain writes info to w. When format is "json" it emits a
// {"explain": ...} envelope; otherwise it prints a plain-text plan.
func printExplain(w io.Writer, info ExplainInfo, format string) error {
	if format == "json" {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(map[string]any{"explain": info})
	}
	return printExplainText(w, info)
}

func printExplainText(w io.Writer, info ExplainInfo) error {
	fmt.Fprintf(w, "Command:     %s\n", info.Command)
	fmt.Fprintf(w, "Description: %s\n", info.Description)

	fmt.Fprintf(w, "\nWhat it reads:\n")
	if len(info.Reads) == 0 {
		fmt.Fprintf(w, "  (nothing)\n")
	} else {
		for _, r := range info.Reads {
			fmt.Fprintf(w, "  • %s\n", r)
		}
	}

	fmt.Fprintf(w, "\nWhat it writes:\n")
	if len(info.Writes) == 0 {
		fmt.Fprintf(w, "  (nothing)\n")
	} else {
		for _, wr := range info.Writes {
			fmt.Fprintf(w, "  • %s\n", wr)
		}
	}

	fmt.Fprintf(w, "\nSide effects and risks:\n")
	if len(info.SideEffects) == 0 {
		fmt.Fprintf(w, "  (none)\n")
	} else {
		for _, se := range info.SideEffects {
			fmt.Fprintf(w, "  • %s\n", se)
		}
	}

	fmt.Fprintf(w, "\nRequired permissions:\n")
	for _, p := range info.Permissions {
		fmt.Fprintf(w, "  • %s\n", p)
	}

	fmt.Fprintf(w, "\nExit codes:\n")
	for _, ec := range info.ExitCodes {
		fmt.Fprintf(w, "  %-4s %s\n", ec.Code, ec.Description)
	}

	return nil
}
