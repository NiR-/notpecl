package cmd

import (
	"context"
	"github.com/NiR-/notpecl/backends"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"os"
	"path"
)

var buildFlags = struct {
	xml string
}{
	xml: "",
}

func NewBuildCmd() *cobra.Command {
	build := &cobra.Command{
		Use:                   "build [--xml=<xml-path>] [<src-path>] -- [<extra-configure-args>]",
		DisableFlagsInUseLine: true,
		DisableAutoGenTag:     true,
		Short:                 "build the extension in the given path or in the current directory if none provided",
		Run:                   runBuildCmd,
	}

	build.Flags().StringVar(&buildFlags.xml, "xml", "", "Path to the package.xml file relative to the given extension path.")

	return build
}

func runBuildCmd(cmd *cobra.Command, args []string) {
	np := backends.NewNotPeclBackend()
	p := initPeclBackend(np, "")
	ctx := context.TODO()

	extDir := cwd()
	if len(args) > 0 {
		extDir = args[0]
		args = args[1:]
	}

	if buildFlags.xml == "" {
		if pathExists(path.Join(extDir, "package.xml")) {
			buildFlags.xml = path.Join(extDir, "package.xml")
		} else if pathExists(path.Join(extDir, "..", "package.xml")) {
			buildFlags.xml = path.Join(extDir, "..", "package.xml")
		} else {
			logrus.Fatalf(
				"No package.xml found in %s, nor in its parent directory.",
				path.Join(extDir, "package.xml"))
		}
	}

	opts := backends.BuildOpts{
		ExtensionDir:   extDir,
		PackageXmlPath: buildFlags.xml,
		ConfigureArgs:  []string{},
		Parallel:       findMaxParallelism(),
	}
	opts.ConfigureArgs = args

	if err := p.Build(ctx, opts); err != nil {
		logrus.Fatal(err)
	}
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
