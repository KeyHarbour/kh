package main

import (
	"os"

	"github.com/KeyHarbour/kh/internal/cli"
)

func main() {
	code := cli.Execute()
	os.Exit(code)
}
