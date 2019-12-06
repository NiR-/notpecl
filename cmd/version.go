package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	// These two variables are defined at compile-time by a flag passed to the
	// go linker (see the Makefile).
	releaseVersion string
	commitHash     string
)

func NewVersionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "version",
		Short:             "Show notpecl version",
		DisableAutoGenTag: true,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("notpecl %s (commit: %s)\n", releaseVersion, commitHash)
		},
	}

	return cmd
}
