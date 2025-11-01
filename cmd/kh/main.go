package main

import (
	"os"

	"kh/internal/cli"
)

func main() {
	code := cli.Execute()
	os.Exit(code)
}
