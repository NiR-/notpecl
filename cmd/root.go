package cmd

import (
	"log"
	"os"
	"path"
	"runtime"

	"github.com/NiR-/notpecl/pecl"
	"github.com/NiR-/notpecl/ui"
	"github.com/mattn/go-isatty"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var rootFlags = struct {
	verbose bool
}{
	verbose: false,
}

func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:               "notpecl",
		DisableAutoGenTag: true,
		Short:             "Download, build and install PHP community extensions",
		PersistentPreRun: func(_ *cobra.Command, _ []string) {
			if rootFlags.verbose {
				logrus.SetLevel(logrus.DebugLevel)
			}
		},
	}
	root.PersistentFlags().BoolVarP(&rootFlags.verbose, "verbose", "v", true, "Use this flag to enable debug log messages.")

	root.AddCommand(NewBuildCmd())
	root.AddCommand(NewDownloadCmd())
	root.AddCommand(NewInstallCmd())

	return root
}

func initPeclBackend() pecl.PeclBackend {
	cacheDir, err := findCacheDir()
	if err != nil {
		log.Fatal(err)
	}

	downloadDir, err := findDownloadDir()
	if err != nil {
		log.Fatal(err)
	}

	peclCacheDir := path.Join(cacheDir, "pecl")
	p, err := pecl.NewPeclBackend(peclCacheDir, downloadDir, "/tmp/notpecl")
	if err != nil {
		log.Fatal(err)
	}

	if isatty.IsTerminal(os.Stdout.Fd()) {
		interactiveUI := ui.NewInteractiveUI(os.Stdin, os.Stdout)
		p = p.WithUI(interactiveUI)
	}

	return p
}

func findCacheDir() (string, error) {
	basepath := os.Getenv("XDG_DATA_HOME")
	if basepath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		basepath = path.Join(home, ".local", "share")
	}
	return path.Join(basepath, "notpecl"), nil
}

func findDownloadDir() (string, error) {
	dir := path.Join(os.TempDir(), "notpecl")
	_, err := os.Stat(dir)
	if os.IsNotExist(err) {
		err = os.Mkdir(dir, 0750)
	}
	return dir, err
}

func findMaxParallelism() int {
	maxProcs := runtime.GOMAXPROCS(0)
	numCPU := runtime.NumCPU()
	if maxProcs < numCPU {
		return maxProcs
	}
	return numCPU
}
