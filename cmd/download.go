package cmd

import (
	"context"
	"log"
	"strings"

	"github.com/NiR-/notpecl/backends"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
)

func NewDownloadCmd() *cobra.Command {
	download := &cobra.Command{
		Use:               "download <extension[:constraint]> ...",
		DisableAutoGenTag: true,
		Short:             "download the given extensions and optionally unpack them",
		Run:               runDownloadCmd,
	}

	return download
}

func runDownloadCmd(cmd *cobra.Command, args []string) {
	np := backends.NewNotPeclBackend()
	p := initPeclBackend(np)
	eg, ctx := errgroup.WithContext(context.TODO())

	if len(args) == 0 {
		logrus.Fatal("You have to provide at least one extension.")
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

			version, err := p.ResolveConstraint(ctx, name, constraint)
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

// php -r "echo ini_get('extension_dir');"
// /usr/local/lib/php/extensions/no-debug-non-zts-20180731/
//
//
