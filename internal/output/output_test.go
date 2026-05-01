package output

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

type failWriter struct{ err error }

func (f failWriter) Write(p []byte) (int, error) { return 0, f.err }

func TestJSON_Basic(t *testing.T) {
	var buf bytes.Buffer
	p := Printer{W: &buf}

	if err := p.JSON(map[string]string{"key": "val"}); err != nil {
		t.Fatal(err)
	}

	var got map[string]string
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, buf.String())
	}
	if got["key"] != "val" {
		t.Fatalf("expected key=val, got %v", got)
	}
}

func TestJSON_Indented(t *testing.T) {
	var buf bytes.Buffer
	p := Printer{W: &buf}

	p.JSON(map[string]int{"n": 1})

	if !strings.Contains(buf.String(), "  ") {
		t.Fatalf("expected indented JSON, got: %s", buf.String())
	}
}

func TestJSON_WriteError(t *testing.T) {
	want := errors.New("write failed")
	p := Printer{W: failWriter{err: want}}

	err := p.JSON(map[string]string{"k": "v"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestTable_TableFormat(t *testing.T) {
	var buf bytes.Buffer
	p := Printer{Format: "table", W: &buf}

	headers := []string{"NAME", "STATUS"}
	rows := [][]string{
		{"alpha", "ok"},
		{"beta", "pending"},
	}

	if err := p.Table(headers, rows); err != nil {
		t.Fatal(err)
	}

	out := buf.String()
	for _, h := range headers {
		if !strings.Contains(out, h) {
			t.Errorf("expected header %q in output", h)
		}
	}
	for _, row := range rows {
		for _, cell := range row {
			if !strings.Contains(out, cell) {
				t.Errorf("expected cell %q in output", cell)
			}
		}
	}
}

func TestTable_EmptyRows(t *testing.T) {
	var buf bytes.Buffer
	p := Printer{Format: "table", W: &buf}

	if err := p.Table([]string{"COL"}, nil); err != nil {
		t.Fatal(err)
	}

	out := buf.String()
	if !strings.Contains(out, "COL") {
		t.Fatalf("expected header in output, got: %s", out)
	}
}

func TestTable_JSONFormat(t *testing.T) {
	var buf bytes.Buffer
	p := Printer{Format: "json", W: &buf}

	headers := []string{"ID", "NAME"}
	rows := [][]string{{"1", "foo"}}

	if err := p.Table(headers, rows); err != nil {
		t.Fatal(err)
	}

	var got struct {
		Headers []string   `json:"headers"`
		Rows    [][]string `json:"rows"`
	}
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, buf.String())
	}
	if len(got.Headers) != 2 || got.Headers[0] != "ID" {
		t.Fatalf("unexpected headers: %v", got.Headers)
	}
	if len(got.Rows) != 1 || got.Rows[0][1] != "foo" {
		t.Fatalf("unexpected rows: %v", got.Rows)
	}
}

func TestTable_SingleColumn(t *testing.T) {
	var buf bytes.Buffer
	p := Printer{Format: "table", W: &buf}

	if err := p.Table([]string{"ITEM"}, [][]string{{"a"}, {"b"}}); err != nil {
		t.Fatal(err)
	}

	out := buf.String()
	if !strings.Contains(out, "a") || !strings.Contains(out, "b") {
		t.Fatalf("expected rows in output, got: %s", out)
	}
}

func TestTable_RowOrder(t *testing.T) {
	var buf bytes.Buffer
	p := Printer{Format: "table", W: &buf}

	p.Table([]string{"N"}, [][]string{{"first"}, {"second"}, {"third"}})

	out := buf.String()
	iFirst := strings.Index(out, "first")
	iSecond := strings.Index(out, "second")
	iThird := strings.Index(out, "third")

	if iFirst >= iSecond || iSecond >= iThird {
		t.Fatalf("expected rows in order, got: %s", out)
	}
}
