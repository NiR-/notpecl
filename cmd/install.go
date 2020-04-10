package cmd

import (
	"context"
	"strings"

	"github.com/NiR-/notpecl/pecl"
	"github.com/NiR-/notpecl/peclapi"
	"github.com/spf13/cobra"
	"golang.org/x/xerrors"
)

var installFlags = struct {
	cleanup          bool
	minimumStability string
	downloadDir      string
	installDir       string
}{
	cleanup: true,
}

func NewInstallCmd() *cobra.Command {
	install := &cobra.Command{
		Use:               "install",
		DisableAutoGenTag: true,
		Short:             "install the given extensions",
		Run:               run(runInstallCmd),
	}

	install.Flags().BoolVar(&installFlags.cleanup,
		"cleanup",
		true,
		"Remove source code and build files after installing the extension (enabled by default).")
	install.Flags().StringVar(&installFlags.minimumStability,
		"minimum-stability",
		peclapi.Stable.String(),
		"Minimum stability level to look for when resolving version constraints (default: stable, available: stable > beta > alpha > devel > snapshot)")
	install.Flags().StringVar(&installFlags.downloadDir,
		"download-dir",
		"",
		"Directory where the extensions should be downloaded and compiled (defaults to a temporary directory).")
	install.Flags().StringVar(&installFlags.installDir,
		"install-dir",
		"",
		"Directory where the extensions shoud be installed.")
	// @TODO: add a flag to set configure args for each extension

	return install
}

func runInstallCmd(cmd *cobra.Command, args []string) error {
	p := initPeclBackend()
	ctx := context.Background()

	stability := peclapi.StabilityFromString(installFlags.minimumStability)
	downloadDir := installFlags.downloadDir
	if downloadDir == "" {
		var err error
		if downloadDir, err = resolveTmpDownloadDir(); err != nil {
			return xerrors.Errorf("failed to find where downloaded files should be written: %w", err)
		}
	}

	for _, arg := range args {
		segments := strings.SplitN(arg, ":", 2)
		extName := segments[0]
		extVerConstraint := "*"
		if len(segments) == 2 {
			extVerConstraint = segments[1]
		}

		extVersion, err := p.ResolveConstraint(ctx, extName, extVerConstraint, stability)
		if err != nil {
			return err
		}

		opts := pecl.InstallOpts{
			DownloadOpts: pecl.DownloadOpts{
				Extension:   extName,
				Version:     extVersion,
				DownloadDir: downloadDir,
			},
			ConfigureArgs: []string{},
			Parallel:      findMaxParallelism(),
			Cleanup:       installFlags.cleanup,
			InstallDir:    installFlags.installDir,
		}
		if err := p.Install(ctx, opts); err != nil {
			return err
		}
	}

	return nil
}
