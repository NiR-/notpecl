package cmd

import (
	"context"
	"log"
	"strings"

	"github.com/NiR-/notpecl/backends"
	"github.com/NiR-/notpecl/extindex"
	"github.com/spf13/cobra"
)

var installFlags = struct {
	cleanup          bool
	minimumStability string
}{
	cleanup: true,
}

func NewInstallCmd() *cobra.Command {
	install := &cobra.Command{
		Use:               "install",
		DisableAutoGenTag: true,
		Short:             "install the given extensions",
		Run:               runInstallCmd,
	}

	install.Flags().BoolVar(&installFlags.cleanup,
		"no-cleanup",
		false,
		"Don't remove source code and build files.",
	)
	install.Flags().StringVar(&installFlags.minimumStability,
		"minimum-stability",
		string(extindex.Stable),
		"Minimum stability level to look for when resolving version constraints (default: stable, available: stable > beta > alpha > devel > snapshot)",
	)
	// @TODO: add a flag to set configure args for each extension

	return install
}

func runInstallCmd(cmd *cobra.Command, args []string) {
	np := backends.NewNotPeclBackend()
	p := initPeclBackend(np)
	ctx := context.Background()

	stability := extindex.StabilityFromString(installFlags.minimumStability)

	for _, arg := range args {
		segments := strings.SplitN(arg, ":", 2)
		extName := segments[0]
		extVerConstraint := "*"
		if len(segments) == 2 {
			extVerConstraint = segments[1]
		}

		extVersion, err := p.ResolveConstraint(ctx, extName, extVerConstraint, stability)
		if err != nil {
			log.Fatal(err)
		}

		opts := backends.InstallOpts{
			Name:          extName,
			Version:       extVersion,
			ConfigureArgs: []string{},
			Parallel:      findMaxParallelism(),
			Cleanup:       installFlags.cleanup,
		}
		if err := p.Install(ctx, opts); err != nil {
			log.Fatal(err)
		}
	}
}
