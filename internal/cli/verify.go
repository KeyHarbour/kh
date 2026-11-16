package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/KeyHarbour/kh/internal/config"
	"github.com/KeyHarbour/kh/internal/khclient"
	"github.com/KeyHarbour/kh/internal/kherrors"
	"github.com/KeyHarbour/kh/internal/output"
	"github.com/KeyHarbour/kh/internal/state"

	"github.com/spf13/cobra"
)

type verifyCheck struct {
	Name   string `json:"name"`
	OK     bool   `json:"ok"`
	Detail string `json:"detail"`
}

// tfStateHeader holds the top-level Terraform state fields verify checks.
type tfStateHeader struct {
	Version *int   `json:"version"`
	Lineage string `json:"lineage"`
	Serial  *int64 `json:"serial"`
}

// verifyStateContent checks that content is a Terraform v4 state with a
// lineage and serial and, when against is non-nil, that it matches that state.
func verifyStateContent(content []byte, against []byte) []verifyCheck {
	var hdr tfStateHeader
	if err := json.Unmarshal(content, &hdr); err != nil {
		return []verifyCheck{{Name: "json", OK: false, Detail: err.Error()}}
	}
	checks := []verifyCheck{{Name: "json", OK: true, Detail: "valid JSON object"}}

	switch {
	case hdr.Version == nil:
		checks = append(checks, verifyCheck{Name: "version", OK: false, Detail: "missing"})
	case *hdr.Version != 4:
		checks = append(checks, verifyCheck{Name: "version", OK: false, Detail: fmt.Sprintf("%d, want 4", *hdr.Version)})
	default:
		checks = append(checks, verifyCheck{Name: "version", OK: true, Detail: "4"})
	}
	if hdr.Lineage == "" {
		checks = append(checks, verifyCheck{Name: "lineage", OK: false, Detail: "missing"})
	} else {
		checks = append(checks, verifyCheck{Name: "lineage", OK: true, Detail: hdr.Lineage})
	}
	switch {
	case hdr.Serial == nil:
		checks = append(checks, verifyCheck{Name: "serial", OK: false, Detail: "missing"})
	case *hdr.Serial < 0:
		checks = append(checks, verifyCheck{Name: "serial", OK: false, Detail: fmt.Sprintf("%d is negative", *hdr.Serial)})
	default:
		checks = append(checks, verifyCheck{Name: "serial", OK: true, Detail: fmt.Sprint(*hdr.Serial)})
	}

	if against == nil {
		return checks
	}
	var local tfStateHeader
	if err := json.Unmarshal(against, &local); err != nil {
		return append(checks, verifyCheck{Name: "local-json", OK: false, Detail: err.Error()})
	}
	if local.Lineage == hdr.Lineage {
		checks = append(checks, verifyCheck{Name: "local-lineage", OK: true, Detail: "matches"})
	} else {
		checks = append(checks, verifyCheck{Name: "local-lineage", OK: false, Detail: fmt.Sprintf("local %q, stored %q", local.Lineage, hdr.Lineage)})
	}
	if local.Serial != nil && hdr.Serial != nil && *local.Serial == *hdr.Serial {
		checks = append(checks, verifyCheck{Name: "local-serial", OK: true, Detail: "matches"})
	} else {
		checks = append(checks, verifyCheck{Name: "local-serial", OK: false, Detail: fmt.Sprintf("local %s, stored %s", serialString(local.Serial), serialString(hdr.Serial))})
	}
	localSum, storedSum := state.SHA256Hex(against), state.SHA256Hex(content)
	if localSum == storedSum {
		checks = append(checks, verifyCheck{Name: "local-checksum", OK: true, Detail: "sha256 " + storedSum})
	} else {
		checks = append(checks, verifyCheck{Name: "local-checksum", OK: false, Detail: fmt.Sprintf("local sha256 %s, stored %s", localSum, storedSum)})
	}
	return checks
}

func serialString(s *int64) string {
	if s == nil {
		return "missing"
	}
	return fmt.Sprint(*s)
}

func newVerifyCmd() *cobra.Command {
	var workspace, againstPath string
	cmd := &cobra.Command{
		Use:   "verify [statefile-uuid]",
		Short: "Check the integrity of a stored Terraform state",
		Long: `Check that a stored statefile is a well-formed Terraform v4 state with a
lineage and serial.

Without a statefile UUID, the latest statefile of --workspace is checked. With
--against, the stored state must also match that local state file: same
lineage, same serial and same SHA256 checksum.

Exits with code 3 (KH-VAL-006) when any check fails.

Examples:
  kh tf state verify --workspace <workspace-uuid>
  kh tf state verify <statefile-uuid> --against ./terraform.tfstate`,
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) > 1 {
				return kherrors.ErrInvalidValue.New("verify takes at most 1 argument: [statefile-uuid]")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadWithEnv()
			if err != nil {
				return kherrors.ErrConfigLoad.Wrapf(err, "failed to load configuration: %s", err)
			}
			var against []byte
			if againstPath != "" {
				against, err = os.ReadFile(againstPath)
				if err != nil {
					return kherrors.ErrInvalidValue.Wrapf(err, "cannot read --against file: %s", err)
				}
			}

			client := khclient.New(cfg)
			var sf khclient.Statefile
			if len(args) == 1 {
				sf, err = client.GetStatefile(cmd.Context(), args[0])
			} else {
				ws := workspaceRefOrEnv(workspace, cfg)
				if ws == "" {
					return kherrors.ErrMissingFlag.New("pass a statefile UUID, or --workspace (or KH_WORKSPACE) to verify its latest statefile")
				}
				if !looksLikeUUID(ws) {
					return kherrors.ErrInvalidValue.Newf("workspace %q is not a valid UUID", ws)
				}
				sf, err = client.GetLastStatefile(cmd.Context(), ws)
			}
			if err != nil {
				return err
			}

			checks := verifyStateContent([]byte(sf.Content), against)
			var failed []string
			for _, c := range checks {
				if !c.OK {
					failed = append(failed, c.Name)
				}
			}

			printer := output.Printer{Format: outputFormat, W: cmd.OutOrStdout()}
			if printer.Format == "json" {
				if err := printer.JSON(map[string]any{"statefile": sf.UUID, "ok": len(failed) == 0, "checks": checks}); err != nil {
					return err
				}
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "Statefile %s\n", sf.UUID)
				rows := make([][]string, 0, len(checks))
				for _, c := range checks {
					result := "ok"
					if !c.OK {
						result = "FAIL"
					}
					rows = append(rows, []string{c.Name, result, c.Detail})
				}
				if err := printer.Table([]string{"CHECK", "RESULT", "DETAIL"}, rows); err != nil {
					return err
				}
			}
			if len(failed) > 0 {
				return kherrors.ErrStateVerifyFailed.Newf("statefile %s failed %d check(s): %s", sf.UUID, len(failed), strings.Join(failed, ", "))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&workspace, "workspace", "", "Workspace UUID whose latest statefile to verify (or KH_WORKSPACE)")
	cmd.Flags().StringVar(&againstPath, "against", "", "Local state file the stored state must match")
	return cmd
}
