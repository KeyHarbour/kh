package cli

import (
	"context"
	"errors"
	"kh/internal/backend"
	"kh/internal/output"
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
			w := backend.NewHTTPWriterWithHeaders(url, headers)
			ctx, cancel := context.WithTimeout(cmd.Context(), 60*time.Second)
			defer cancel()
			obj, err := w.Put(ctx, url, b, true)
			if err != nil {
				return err
			}
			return printer.JSON(map[string]any{
				"action":   "http.upload-state",
				"url":      obj.URL,
				"bytes":    obj.Size,
				"checksum": obj.Checksum,
				"file":     filePath,
			})
		},
	}
	cmd.Flags().StringVar(&filePath, "file", "", "Path to local .tfstate file")
	cmd.Flags().StringVar(&url, "url", "", "Destination HTTP URL (e.g., http://localhost:8080/states/{module}/{workspace}.tfstate)")
	cmd.Flags().StringVar(&idempotencyKey, "idempotency-key", "", "Optional Idempotency-Key header")
	cmd.Flags().StringVar(&contentType, "content-type", "application/vnd.terraform.state+json;version=4", "Content-Type header for PUT")
	return cmd
}
