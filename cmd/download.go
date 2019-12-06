package cmd

import (
	"context"
	"log"
	"strings"

	"github.com/NiR-/notpecl/backends"
	"github.com/NiR-/notpecl/extindex"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
)

var downloadFlags = struct {
	minimumStability string
}{}

func NewDownloadCmd() *cobra.Command {
	download := &cobra.Command{
		Use:               "download <extension[:constraint]> ...",
		DisableAutoGenTag: true,
		Short:             "download the given extensions and optionally unpack them",
		Run:               runDownloadCmd,
	}

	download.Flags().StringVar(&downloadFlags.minimumStability,
		"minimum-stability",
		string(extindex.Stable),
		"Minimum stability level to look for when resolving version constraints (default: stable, available: stable > beta > alpha > devel > snapshot)",
	)

	return download
}

func runDownloadCmd(cmd *cobra.Command, args []string) {
	np := backends.NewNotPeclBackend()
	p := initPeclBackend(np, "")
	eg, ctx := errgroup.WithContext(context.TODO())

	if len(args) == 0 {
		logrus.Fatal("You have to provide at least one extension.")
	}

	stability := extindex.StabilityFromString(downloadFlags.minimumStability)

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

			extDir, err := p.Download(ctx, name, version)
			if err != nil {
				return err
			}

			logrus.Infof("Extension %s downloaded to %q", name, extDir)
			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		log.Fatal(err)
	}
}
