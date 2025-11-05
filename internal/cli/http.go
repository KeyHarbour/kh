package cli

import (
	"context"
	"errors"
	"fmt"
	"kh/internal/backend"
	"kh/internal/output"
	"kh/internal/state"
	"os"
	"time"

	"github.com/spf13/cobra"
)

func newHTTPCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "http", Short: "HTTP utilities"}
	cmd.AddCommand(newHTTPUploadStateCmd())
	return cmd
}

func newHTTPUploadStateCmd() *cobra.Command {
	var filePath, url, idempotencyKey, contentType string
	var verifyAfter bool

	cmd := &cobra.Command{
		Use:   "upload-state",
		Short: "Upload a local .tfstate file to an HTTP endpoint via PUT",
		RunE: func(cmd *cobra.Command, args []string) error {
			printer := output.Printer{Format: outputFormat, W: cmd.OutOrStdout()}
			if filePath == "" {
				return errors.New("--file is required")
			}
			if url == "" {
				return errors.New("--url is required")
			}
			b, err := os.ReadFile(filePath)
			if err != nil {
				return err
			}
			headers := map[string]string{}
			if idempotencyKey != "" {
				headers["Idempotency-Key"] = idempotencyKey
			}
			if contentType != "" {
				headers["Content-Type"] = contentType
			}
			// If requested, include checksum header so the server can validate content.
			// The server may echo this header back; when echoed we skip the GET read-back verification.
			localSum := ""
			if verifyAfter {
				localSum = state.SHA256Hex(b)
				headers["X-Checksum-Sha256"] = localSum
			}
			w := backend.NewHTTPWriterWithHeaders(url, headers)
			ctx, cancel := context.WithTimeout(cmd.Context(), 60*time.Second)
			defer cancel()
			obj, err := w.Put(ctx, url, b, true)
			if err != nil {
				return err
			}
			// Optionally verify: prefer server-echoed checksum (already present in obj.Checksum).
			if verifyAfter {
				if obj.Checksum != "" {
					// server echoed a checksum; ensure it matches local checksum we sent
					if localSum != "" && obj.Checksum != localSum {
						return fmt.Errorf("verification failed: server checksum mismatch (local=%s server=%s)", localSum, obj.Checksum)
					}
					// server validated and echoed same checksum -> skip read-back
				} else {
					// No server echo; fall back to GET/read-back verification
					r := backend.NewHTTPReader(url)
					_, got, err := r.Get(ctx, url)
					if err != nil {
						return fmt.Errorf("verification failed: read-back error: %w", err)
					}
					if obj.Checksum != got.Checksum {
						return fmt.Errorf("verification failed: checksum mismatch (put=%s get=%s)", obj.Checksum, got.Checksum)
					}
				}
			}
			// Determine whether the server validated the upload
			serverValidated := false
			if verifyAfter {
				if obj.Checksum != "" && localSum != "" {
					serverValidated = (obj.Checksum == localSum)
				} else if obj.Checksum == "" {
					// We performed get/read-back verification above; if checks passed then treat as validated
					serverValidated = true
				}
			}
			return printer.JSON(map[string]any{
				"action":           "http.upload-state",
				"url":              obj.URL,
				"bytes":            obj.Size,
				"checksum":         obj.Checksum,
				"file":             filePath,
				"server_validated": serverValidated,
			})
		},
	}
	cmd.Flags().StringVar(&filePath, "file", "", "Path to local .tfstate file")
	cmd.Flags().StringVar(&url, "url", "", "Destination HTTP URL (e.g., http://localhost:8080/states/{module}/{workspace}.tfstate)")
	cmd.Flags().StringVar(&idempotencyKey, "idempotency-key", "", "Optional Idempotency-Key header")
	cmd.Flags().StringVar(&contentType, "content-type", "application/vnd.terraform.state+json;version=4", "Content-Type header for PUT")
	cmd.Flags().BoolVar(&verifyAfter, "verify-after-upload", true, "Read the uploaded URL back and verify SHA-256 checksum (also sends X-Checksum-Sha256 header when enabled)")
	return cmd
}
