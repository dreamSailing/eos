package main

import (
	"fmt"
	"os"

	"github.com/dreamSailing/vb-coding/internal/cli"
	_ "github.com/dreamSailing/vb-coding/internal/pkg/utils"
)

func main() {
	if err := cli.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
