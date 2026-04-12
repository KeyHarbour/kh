package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"
)

// ── printExplain formatter ────────────────────────────────────────────────────

func TestPrintExplain_Text_ContainsAllSections(t *testing.T) {
	info := ExplainInfo{
		Command:     "kh tf version last",
		Description: "Retrieves the most recently uploaded Terraform statefile for a workspace.",
		Reads:       []string{"GET /workspaces/{uuid}/statefiles/last"},
		SideEffects: []string{"No writes performed."},
		Permissions: []string{"read:statefiles on the target workspace"},
		ExitCodes: []ExplainExit{
			{Code: "0", Description: "Success"},
			{Code: "1", Description: "Workspace or statefile not found"},
		},
	}

	var buf bytes.Buffer
	if err := printExplain(&buf, info, "table"); err != nil {
		t.Fatalf("printExplain error: %v", err)
	}
	out := buf.String()

	for _, want := range []string{
		"kh tf version last",
		"Retrieves the most recently",
		"GET /workspaces/{uuid}/statefiles/last",
		"No writes performed.",
		"read:statefiles",
		"0", "Success",
		"1", "Workspace or statefile not found",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("text output missing %q", want)
		}
	}
}

func TestPrintExplain_Text_EmptyReadsAndWrites(t *testing.T) {
	info := ExplainInfo{
		Command:     "kh kv set",
		Description: "Creates a key/value pair.",
		Writes:      []string{"POST /workspaces/{uuid}/keyvalues"},
		Permissions: []string{"write:keyvalues on the target workspace"},
		ExitCodes:   []ExplainExit{{Code: "0", Description: "Success"}},
	}

	var buf bytes.Buffer
	_ = printExplain(&buf, info, "table")
	out := buf.String()

	if !strings.Contains(out, "(nothing)") {
		t.Error("expected '(nothing)' placeholder for empty reads")
	}
	if strings.Count(out, "(nothing)") != 1 {
		t.Error("expected exactly one '(nothing)' — writes section should show the value")
	}
	if !strings.Contains(out, "POST /workspaces/{uuid}/keyvalues") {
		t.Error("writes section missing")
	}
}

func TestPrintExplain_Text_NoSideEffects(t *testing.T) {
	info := ExplainInfo{
		Command:     "kh tf version last",
		Description: "Read-only.",
		Reads:       []string{"GET /endpoint"},
		Permissions: []string{"read access"},
		ExitCodes:   []ExplainExit{{Code: "0", Description: "Success"}},
	}

	var buf bytes.Buffer
	_ = printExplain(&buf, info, "table")
	out := buf.String()

	if !strings.Contains(out, "(none)") {
		t.Error("expected '(none)' placeholder for empty side effects")
	}
}

func TestPrintExplain_JSON_Envelope(t *testing.T) {
	info := ExplainInfo{
		Command:     "kh tf sync",
		Description: "Reads and writes state.",
		Reads:       []string{"TFC backend"},
		Writes:      []string{"KeyHarbour workspace"},
		SideEffects: []string{"Uploads statefile versions"},
		Permissions: []string{"read:statefiles", "write:statefiles"},
		ExitCodes: []ExplainExit{
			{Code: "0", Description: "Success"},
			{Code: "2", Description: "Partial failure"},
		},
	}

	var buf bytes.Buffer
	if err := printExplain(&buf, info, "json"); err != nil {
		t.Fatalf("printExplain error: %v", err)
	}

	var envelope map[string]any
	if err := json.Unmarshal(buf.Bytes(), &envelope); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, buf.String())
	}

	explainRaw, ok := envelope["explain"]
	if !ok {
		t.Fatal("JSON output missing top-level 'explain' key")
	}
	explainMap, ok := explainRaw.(map[string]any)
	if !ok {
		t.Fatalf("'explain' is %T, want map", explainRaw)
	}

	if explainMap["command"] != "kh tf sync" {
		t.Errorf("command = %v, want 'kh tf sync'", explainMap["command"])
	}
	reads, _ := explainMap["reads"].([]any)
	if len(reads) != 1 {
		t.Errorf("reads len = %d, want 1", len(reads))
	}
	exitCodes, _ := explainMap["exit_codes"].([]any)
	if len(exitCodes) != 2 {
		t.Errorf("exit_codes len = %d, want 2", len(exitCodes))
	}
}

func TestPrintExplain_JSON_OmitsEmptySlices(t *testing.T) {
	info := ExplainInfo{
		Command:     "kh tf version last",
		Description: "Read-only.",
		Reads:       []string{"GET /endpoint"},
		// Writes and SideEffects intentionally absent
		Permissions: []string{"read access"},
		ExitCodes:   []ExplainExit{{Code: "0", Description: "Success"}},
	}

	var buf bytes.Buffer
	_ = printExplain(&buf, info, "json")

	var envelope map[string]any
	_ = json.Unmarshal(buf.Bytes(), &envelope)
	explainMap := envelope["explain"].(map[string]any)

	if _, ok := explainMap["writes"]; ok {
		t.Error("'writes' key should be omitted when empty")
	}
	if _, ok := explainMap["side_effects"]; ok {
		t.Error("'side_effects' key should be omitted when empty")
	}
}

// ── command integration: kh tf version last --explain ─────────────────────────

func TestStatefilesLastExplain_Text(t *testing.T) {
	var buf bytes.Buffer
	cmd := newStatefilesCmd()
	cmd.SetOut(&buf)
	cmd.SetErr(io.Discard)
	cmd.SetContext(context.Background())
	cmd.SetArgs([]string{"last", "--explain"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("version last --explain failed: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "kh tf version last") {
		t.Error("explain output missing command name")
	}
	if !strings.Contains(out, "GET /workspaces/{uuid}/statefiles/last") {
		t.Error("explain output missing reads entry")
	}
	if !strings.Contains(out, "read:statefiles") {
		t.Error("explain output missing permissions")
	}
}

func TestStatefilesLastExplain_JSON(t *testing.T) {
	orig := outputFormat
	outputFormat = "json"
	t.Cleanup(func() { outputFormat = orig })

	var buf bytes.Buffer
	cmd := newStatefilesCmd()
	cmd.SetOut(&buf)
	cmd.SetErr(io.Discard)
	cmd.SetContext(context.Background())
	cmd.SetArgs([]string{"last", "--explain"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("version last --explain -o json failed: %v", err)
	}

	var envelope map[string]any
	if err := json.Unmarshal(buf.Bytes(), &envelope); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, buf.String())
	}
	if _, ok := envelope["explain"]; !ok {
		t.Error("JSON output missing 'explain' key")
	}
}

// ── command integration: kh kv set --explain ──────────────────────────────────

func TestKVSetExplain_Text(t *testing.T) {
	var buf bytes.Buffer
	cmd := newKVCmd()
	cmd.SetOut(&buf)
	cmd.SetErr(io.Discard)
	cmd.SetContext(context.Background())
	cmd.SetArgs([]string{"set", "--explain"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("kv set --explain failed: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "kh kv set") {
		t.Error("explain output missing command name")
	}
	if !strings.Contains(out, "POST /workspaces/{uuid}/keyvalues") {
		t.Error("explain output missing writes entry")
	}
	if !strings.Contains(out, "write:keyvalues") {
		t.Error("explain output missing permissions")
	}
}

func TestKVSetExplain_PrivateFlag_AppearsInSideEffects(t *testing.T) {
	var buf bytes.Buffer
	cmd := newKVCmd()
	cmd.SetOut(&buf)
	cmd.SetErr(io.Discard)
	cmd.SetContext(context.Background())
	cmd.SetArgs([]string{"set", "--explain", "--private"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("kv set --explain --private failed: %v", err)
	}

	if !strings.Contains(buf.String(), "private") {
		t.Error("expected '--private' to appear in side effects when flag is set")
	}
}

// ── command integration: kh tf sync --explain ─────────────────────────────────

func TestSyncExplain_Text_DefaultTo(t *testing.T) {
	var buf bytes.Buffer
	cmd := newSyncCmd()
	cmd.SetOut(&buf)
	cmd.SetErr(io.Discard)
	cmd.SetContext(context.Background())
	cmd.SetArgs([]string{"--from=local", "--explain"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("sync --explain failed: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "local") {
		t.Error("explain should reflect --from=local")
	}
	// Default --to is keyharbour
	if !strings.Contains(out, "keyharbour") {
		t.Error("explain should reflect default --to=keyharbour")
	}
}

func TestSyncExplain_JSON(t *testing.T) {
	orig := outputFormat
	outputFormat = "json"
	t.Cleanup(func() { outputFormat = orig })

	var buf bytes.Buffer
	cmd := newSyncCmd()
	cmd.SetOut(&buf)
	cmd.SetErr(io.Discard)
	cmd.SetContext(context.Background())
	cmd.SetArgs([]string{"--from=tfc", "--to=keyharbour", "--explain"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("sync --explain -o json failed: %v", err)
	}

	var envelope map[string]any
	if err := json.Unmarshal(buf.Bytes(), &envelope); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, buf.String())
	}
	if _, ok := envelope["explain"]; !ok {
		t.Error("JSON output missing 'explain' key")
	}
	explainMap := envelope["explain"].(map[string]any)
	if !strings.Contains(explainMap["command"].(string), "tfc") {
		t.Error("command field should include --from=tfc")
	}
}

func TestSyncExplain_WithOverwriteAndLock_AppearsInSideEffects(t *testing.T) {
	var buf bytes.Buffer
	cmd := newSyncCmd()
	cmd.SetOut(&buf)
	cmd.SetErr(io.Discard)
	cmd.SetContext(context.Background())
	cmd.SetArgs([]string{"--from=keyharbour", "--explain", "--overwrite", "--lock"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("sync --explain --overwrite --lock failed: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "overwrite") {
		t.Error("expected 'overwrite' in side effects")
	}
	if !strings.Contains(out, "lock") {
		t.Error("expected 'lock' in side effects")
	}
}
