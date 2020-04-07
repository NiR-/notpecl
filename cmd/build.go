package cmd

import (
	"context"
	"os"
	"path/filepath"

	"github.com/NiR-/notpecl/pecl"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/xerrors"
)

var buildFlags = struct {
	xml     string
	cleanup bool
}{}

func NewBuildCmd() *cobra.Command {
	build := &cobra.Command{
		Use:                   "build [--xml=<xml-path>] [<src-path>] -- [<extra-configure-args>]",
		DisableFlagsInUseLine: true,
		DisableAutoGenTag:     true,
		Short:                 "Build an extension from the given path or the current directory if none provided",
		RunE:                  runBuildCmd,
	}

	build.Flags().StringVar(&buildFlags.xml, "xml", "", "Path to the package.xml file relative to the given source path.")
	build.Flags().BoolVar(&buildFlags.cleanup,
		"cleanup",
		true,
		"Remove build files after building the extension (enabled by default).")

	return build
}

func runBuildCmd(cmd *cobra.Command, args []string) error {
	extDir := cwd()
	if len(args) > 0 {
		extDir = args[0]
		args = args[1:]
	}

	if buildFlags.xml == "" {
		if pathExists(filepath.Join(extDir, "package.xml")) {
			buildFlags.xml = filepath.Join(extDir, "package.xml")
		} else if pathExists(filepath.Join(extDir, "..", "package.xml")) {
			buildFlags.xml = filepath.Join(extDir, "..", "package.xml")
		} else {
			return xerrors.Errorf(
				"no package.xml found in %s, nor in its parent directory",
				filepath.Join(extDir, "package.xml"))
		}
	}

	opts := pecl.BuildOpts{
		SourceDir:      extDir,
		PackageXmlPath: buildFlags.xml,
		ConfigureArgs:  []string{},
		Parallel:       findMaxParallelism(),
		Cleanup:        buildFlags.cleanup,
	}
	opts.ConfigureArgs = args

	ctx := context.TODO()
	p := initPeclBackend()

	return p.Build(ctx, opts)
}

func cwd() string {
	cwd, err := os.Getwd()
	if err != nil {
		logrus.Fatal(err)
	}
	return cwd
}

func pathExists(fullpath string) bool {
	_, err := os.Stat(fullpath)
	return err != nil
}
