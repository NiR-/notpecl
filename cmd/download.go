package cmd

import (
	"context"
	"strings"

	"github.com/NiR-/notpecl/pecl"
	"github.com/NiR-/notpecl/peclapi"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
	"golang.org/x/xerrors"
)

var downloadFlags = struct {
	minimumStability string
}{}

func NewDownloadCmd() *cobra.Command {
	download := &cobra.Command{
		Use:               "download <extension[:constraint]> ...",
		DisableAutoGenTag: true,
		Short:             "download the given extensions and optionally unpack them",
		RunE:              runDownloadCmd,
	}

	download.Flags().StringVar(&downloadFlags.minimumStability,
		"minimum-stability",
		string(peclapi.Stable),
		"Minimum stability level to look for when resolving version constraints (default: stable, available: stable > beta > alpha > devel > snapshot)",
	)

	return download
}

func runDownloadCmd(cmd *cobra.Command, args []string) error {
	p := initPeclBackend()

	if len(args) == 0 {
		return xerrors.Errorf("you have to provide at least one extension")
	}

	eg, ctx := errgroup.WithContext(context.TODO())
	stability := peclapi.StabilityFromString(downloadFlags.minimumStability)
	downloadDir, err := findDownloadDir()
	if err != nil {
		return xerrors.Errorf("failed to find where downloaded files should be written: %w", err)
	}

	for i := range args {
		ext := args[i]
		eg.Go(func() error {
			segments := strings.SplitN(ext, ":", 2)
			name := segments[0]
			constraint := "*"
			if len(segments) == 2 {
				constraint = segments[1]
			}

			version, err := p.ResolveConstraint(ctx, name, constraint, stability)
			if err != nil {
				return err
			}

			opts := pecl.DownloadOpts{
				Extension:   name,
				Version:     version,
				DownloadDir: downloadDir,
			}
			extDir, err := p.Download(ctx, opts)
			if err != nil {
				return err
			}

			logrus.Infof("Extension %s downloaded to %q", name, extDir)
			return nil
		})
	}

	return eg.Wait()
}
