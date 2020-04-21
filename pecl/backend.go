package pecl

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/NiR-/notpecl/cmdexec"
	"github.com/NiR-/notpecl/peclapi"
	"github.com/NiR-/notpecl/peclpkg"
	"github.com/NiR-/notpecl/ui"
	"github.com/mcuadros/go-version"
	"github.com/sirupsen/logrus"
	"github.com/twpayne/go-vfs"
	"golang.org/x/xerrors"
)

type Backend interface {
	ResolveConstraint(name, constraint string, minimumStability peclapi.Stability) (string, error)
	Install(opts InstallOpts) error
	Download(opts DownloadOpts) (string, error)
	Build(opts BuildOpts) error
}

type backend struct {
	ui            ui.UI
	apiClient     peclapi.Client
	fs            vfs.FS
	cmdexec       cmdexec.CmdExecutor
	phpConfigPath string
}

// New creates a new pecl backend with d default (and fully working) peclapi
// client, a default vfs.FS instance, a default command runner (used to compile
// the extensions) and a non interactive UI. All these default values can be
// changed through With*() functions.
func New(opts ...BackendOpt) backend {
	b := backend{
		ui:        ui.NewNonInteractiveUI(),
		apiClient: peclapi.NewClient(),
		fs:        vfs.HostOSFS,
		cmdexec:   cmdexec.NewExecutor(),
	}
	for _, opt := range opts {
		opt(&b)
	}

	return b
}

type BackendOpt func(b *backend)

// WithUI returns a new BackendOpt that could be used with New() to change the
// default ui.UI used.
func WithUI(ui ui.UI) BackendOpt {
	return func(b *backend) {
		b.ui = ui
	}
}

// WithClient returns a new BackendOpt that could be used with New() to change
// the default peclapi.Client used.
func WithClient(client peclapi.Client) BackendOpt {
	return func(b *backend) {
		b.apiClient = client
	}
}

// WithFS returns a BackendOpt that could be used with New() to change the
// default instance of vfs.FS to a custom instance.
func WithFS(fs vfs.FS) BackendOpt {
	return func(b *backend) {
		b.fs = fs
	}
}

// WithCmdExec returns a BackendOpt that could be used with New() to change
// the default instance of CmdExec used by the backend.
func WithCmdExec(cmdexec cmdexec.CmdExecutor) BackendOpt {
	return func(b *backend) {
		b.cmdexec = cmdexec
	}
}

func WithPhpConfigPath(phpConfigPath string) BackendOpt {
	return func(b *backend) {
		b.phpConfigPath = phpConfigPath
	}
}

// ResolveConstraint takes an extension name, a version constraint in
// Composer format and also the minimum stability accepted. It tries to find
// a release of that extension that statifies the version constraint and the
// minimum stability.
func (b backend) ResolveConstraint(
	name,
	constraint string,
	minimumStability peclapi.Stability,
) (string, error) {
	extVersions, err := b.apiClient.ListReleases(name)
	if err != nil {
		return "", xerrors.Errorf("could not resolve constraint for %s: %w", name, err)
	}

	cg := version.NewConstrainGroupFromString(constraint)
	sortedVersions := extVersions.Sort()

	for i := 0; i < len(sortedVersions); i++ {
		extVer := sortedVersions[i]
		stability := extVersions[extVer]
		if stability < minimumStability {
			continue
		}
		if cg.Match(extVer) {
			return extVer, nil
		}
	}

	return "", xerrors.Errorf("could not find a version of %s satisfying %q", name, constraint)
}

type InstallOpts struct {
	DownloadOpts

	// InstallDir is the directory where the compiled extension is copied to.
	InstallDir string
	// ConfigureArgs is a list of flags to pass to ./configure when building
	// the extension.
	ConfigureArgs []string
	// Parallel is the maximum number of parallel jobs executed by make at once.
	Parallel int
	// Clenaup indicates whether source code and build files should be removed
	// after sucessful builds.
	Cleanup bool
}

func (b backend) Install(opts InstallOpts) error {
	extDir, err := b.Download(opts.DownloadOpts)
	if err != nil {
		return err
	}

	buildOpts := BuildOpts{
		SourceDir:      extDir,
		InstallDir:     opts.InstallDir,
		PackageXmlPath: filepath.Join(extDir, "package.xml"),
		ConfigureArgs:  opts.ConfigureArgs,
		Parallel:       opts.Parallel,
		Cleanup:        opts.Cleanup,
	}
	if err := b.Build(buildOpts); err != nil {
		return xerrors.Errorf("failed to install %s: %w", opts.DownloadOpts.Extension, err)
	}

	if opts.Cleanup {
		if err := b.fs.RemoveAll(extDir); err != nil {
			return xerrors.Errorf("failed to install %s: %w", opts.DownloadOpts.Extension, err)
		}
	}

	return nil
}

type DownloadOpts struct {
	// Extension is the name of the extension to download.
	Extension string
	// Version is the exact version number of the extension to download.
	Version string
	// DownloadDir is the directory where notpecl decompress downloaded
	// extension archives.
	DownloadDir string
}

func (b backend) Download(opts DownloadOpts) (string, error) {
	dirPrefix := fmt.Sprintf("%s-%s/", opts.Extension, opts.Version)
	extDir := filepath.Join(opts.DownloadDir, dirPrefix)
	if _, err := b.fs.Stat(extDir); err == nil {
		return extDir, nil
	}

	release, err := b.apiClient.DescribeRelease(opts.Extension, opts.Version)
	if err != nil {
		return "", xerrors.Errorf("failed to download %s v%s: %w", opts.Extension, opts.Version, err)
	}

	rawr, err := b.apiClient.DownloadRelease(release)
	if err != nil {
		return "", xerrors.Errorf("failed to download %s v%s: %w", opts.Extension, opts.Version, err)
	}

	gzipr, err := gzip.NewReader(rawr)
	if err != nil {
		return "", xerrors.Errorf("could not decompress %s v%s: %w", opts.Extension, opts.Version, err)
	}
	defer gzipr.Close()

	tarr := tar.NewReader(gzipr)

	for {
		headers, err := tarr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", xerrors.Errorf("could not decompress %s v%s: %w", opts.Extension, opts.Version, err)
		}

		switch headers.Typeflag {
		case tar.TypeReg:
			if err := b.extractFile(extDir, dirPrefix, tarr, headers); err != nil {
				return "", xerrors.Errorf("could not decompress %s v%s: %w", opts.Extension, opts.Version, err)
			}
		}
	}

	return extDir, nil
}

func (b backend) extractFile(
	extDir string,
	dirPrefix string,
	tarr *tar.Reader,
	headers *tar.Header,
) error {
	fullpath := filepath.Join(extDir, headers.Name)
	if strings.HasPrefix(headers.Name, dirPrefix) {
		trimmedPath := strings.TrimPrefix(headers.Name, dirPrefix)
		fullpath = filepath.Join(extDir, trimmedPath)
	}

	dirpath := filepath.Dir(fullpath)
	if err := vfs.MkdirAll(b.fs, dirpath, 0750); err != nil {
		return xerrors.Errorf("could not extract %s: could not create %s: %w", headers.Name, dirpath, err)
	}

	f, err := b.fs.Create(fullpath)
	if err != nil {
		return xerrors.Errorf("could not extract %s: %w", headers.Name, err)
	}
	defer f.Close()

	var total int64
	logrus.Debugf("Unpacking %s (%d bytes)...", headers.Name, headers.Size)

	for {
		var isEOF bool
		raw := make([]byte, headers.Size-total)
		r, err := tarr.Read(raw)
		if err == io.EOF {
			isEOF = true
		} else if err != nil {
			return xerrors.Errorf("could not extract %s: failed to read from tar archive: %w", headers.Name, err)
		}

		_, err = f.Write(raw[:r])
		if err != nil {
			return xerrors.Errorf("could not extract %s: failed to write extracted file: %w", headers.Name, err)
		}

		total += int64(r)
		if isEOF {
			break
		}
	}

	if total != headers.Size {
		return xerrors.Errorf("file %q is %d bytes long, but only %d read from tar archive", headers.Name, headers.Size, total)
	}

	return nil
}

type BuildOpts struct {
	// SourceDir is the folder containing the source code of the extension to
	// build.
	SourceDir string
	// InstallDir is the folder where the compiled extension should be installed.
	InstallDir string
	// PackageXmlPath is the full path to the package.xml for the extension to
	// build.
	PackageXmlPath string
	// ConfigureArgs is a list of flags to pass to ./configure when building.
	ConfigureArgs []string
	// Parallel is the maximum number of parallel jobs executed by make at once.
	Parallel int
	// Cleanup indicates whether make clean should be run.
	Cleanup bool
}

func (b backend) Build(opts BuildOpts) error {
	var err error
	logrus.Debugf("Loading %s...", opts.PackageXmlPath)

	xmlPath, err := b.fs.RawPath(opts.PackageXmlPath)
	if err != nil {
		return xerrors.Errorf("failed to build package: %w", err)
	}

	pkg, err := peclpkg.LoadPackageXMLFromFile(xmlPath)
	if err != nil {
		return xerrors.Errorf("failed to load package.xml: %v", err)
	}

	sourceDir, err := b.fs.RawPath(opts.SourceDir)
	if err != nil {
		return xerrors.Errorf("failed to build %s: %w", pkg.Name, err)
	}

	cmdexec := b.cmdexec.With(
		cmdexec.BaseDir(sourceDir),
		cmdexec.ExtraEnv([]string{
			"PATH=" + lookupEnv("PATH", ""),
			"CFLAGS=" + lookupEnv("PHP_CFLAGS", defaultCflags),
			"CPPFLAGS=" + lookupEnv("PHP_CPPFLAGS", defaultCppflags),
			"LDFLAGS=" + lookupEnv("PHP_LDFLAGS", defaultLdflags),
		}))

	modulePath := filepath.Join(opts.SourceDir, fmt.Sprintf("modules/%s.so", pkg.Name))
	if _, err := b.fs.Stat(modulePath); os.IsNotExist(err) {
		if err := b.checkPackageDependencies(pkg); err != nil {
			return err
		}
		if err := askAboutMissingArgs(b.ui, pkg, &opts); err != nil {
			return err
		}

		if err := b.buildStepPhpize(cmdexec); err != nil {
			return err
		}

		if err := b.buildStepConfigure(cmdexec, opts, pkg); err != nil {
			return err
		}

		if err := b.buildStepMake(cmdexec); err != nil {
			return err
		}
	}

	if err := b.buildStepMakeInstall(cmdexec, opts.InstallDir); err != nil {
		return err
	}

	if opts.Cleanup {
		if err := b.buildStepMakeClean(cmdexec); err != nil {
			return err
		}
	}

	return nil
}

func (b backend) buildStepPhpize(cmdexec cmdexec.CmdExecutor) error {
	logrus.Debug("Running phpize...")

	if err := cmdexec.Run("phpize"); err != nil {
		return xerrors.Errorf("failed to run phpize: %v", err)
	}

	return nil
}

func (b backend) buildStepConfigure(cmdexec cmdexec.CmdExecutor, opts BuildOpts, pkg peclpkg.Package) error {
	logrus.Debug("Running ./configure...")

	if b.phpConfigPath == "" {
		if err := b.resolvePhpConfigPath(); err != nil {
			return xerrors.Errorf("failed to build %s: %w", pkg.Name, err)
		}
	}

	args := append(opts.ConfigureArgs, "--with-php-config="+b.phpConfigPath)
	err := cmdexec.Run("./configure", args...)
	if err != nil {
		return xerrors.Errorf("failed to run configure: %v", err)
	}

	return nil
}

func (b backend) resolvePhpConfigPath() error {
	var err error
	b.phpConfigPath, err = exec.LookPath("php-config")

	return err
}

func (b backend) buildStepMake(cmdexec cmdexec.CmdExecutor) error {
	logrus.Debug("Running make...")

	if err := cmdexec.Run("make"); err != nil {
		return xerrors.Errorf("failed to run make: %v", err)
	}

	return nil
}

func (b backend) buildStepMakeInstall(cmdexec cmdexec.CmdExecutor, installDir string) error {
	logrus.Debug("Running make install...")

	installArgs := make([]string, 0, 2)
	if installDir != "" {
		installArgs = append(installArgs, "INSTALL_ROOT="+installDir)
	}
	installArgs = append(installArgs, "install")

	if err := cmdexec.Run("make", installArgs...); err != nil {
		return xerrors.Errorf("failed to run make install: %v", err)
	}

	return nil
}

func (b backend) buildStepMakeClean(cmdexec cmdexec.CmdExecutor) error {
	logrus.Debug("Running make clean...")

	if err := cmdexec.Run("make", "clean"); err != nil {
		return xerrors.Errorf("failed to run make clean: %v", err)
	}

	return nil
}

var (
	defaultCflags   = "-fstack-protector-strong -fpic -fpie -O2 -D_LARGEFILE_SOURCE -D_FILE_OFFSET_BITS=64"
	defaultCppflags = "-fstack-protector-strong -fpic -fpie -O2 -D_LARGEFILE_SOURCE -D_FILE_OFFSET_BITS=64"
	defaultLdflags  = "-Wl,-O1 -Wl,--hash-style=both -pie"
)

func lookupEnv(name, defaultVal string) string {
	val, _ := os.LookupEnv(name)
	return val
}

func (b backend) checkPackageDependencies(pkg peclpkg.Package) error {
	logrus.Debug("Checking extension dependencies...")
	if err := b.checkPHPVersion(pkg.Dependencies.Required.PHP); err != nil {
		return err
	}

	for _, dep := range pkg.Dependencies.Required.Extensions {
		isEnabled, err := b.isExtensionEnabled(dep.Name)
		if err != nil {
			return err
		}
		if !isEnabled {
			return xerrors.Errorf("Extension %q is required but is not enabled.", dep.Name)
		}
	}

	for _, dep := range pkg.Dependencies.Optional.Extensions {
		isEnabled, err := b.isExtensionEnabled(dep.Name)
		if err != nil {
			return err
		}
		if !isEnabled {
			logrus.Infof("Optional extension %q is not enabled.", dep.Name)
		}
	}

	return nil
}

func (b backend) checkPHPVersion(extConstraint peclpkg.PHPConstraint) error {
	cg := version.NewConstrainGroup()
	if extConstraint.Min != "" {
		cg.AddConstraint(version.NewConstrain(">=", extConstraint.Min))
	}
	if extConstraint.Max != "" {
		cg.AddConstraint(version.NewConstrain("<=", extConstraint.Max))
	}

	for _, excluded := range extConstraint.Exclude {
		cg.AddConstraint(version.NewConstrain("!=", excluded))
	}

	currentVersion, err := b.currentPHPVersion()
	if err != nil {
		return err
	}

	if !cg.Match(currentVersion) {
		return xerrors.Errorf(
			"current php version is %s, required >=%s,<=%s (excluded: %v)",
			currentVersion,
			extConstraint.Min,
			extConstraint.Max,
			extConstraint.Exclude,
		)
	}

	return nil
}

func (b backend) currentPHPVersion() (string, error) {
	var outbuf bytes.Buffer
	cmdexec := b.cmdexec.With(cmdexec.Stdout(&outbuf))

	err := cmdexec.Run("php", "-r", "echo json_encode(PHP_VERSION);")
	if err != nil {
		return "", err
	}

	var phpVersion string
	if err := json.Unmarshal(outbuf.Bytes(), &phpVersion); err != nil {
		return "", err
	}

	return phpVersion, nil
}

func (b backend) isExtensionEnabled(name string) (bool, error) {
	var outbuf bytes.Buffer
	cmdexec := b.cmdexec.With(cmdexec.Stdout(&outbuf))
	err := cmdexec.Run("php", "-r",
		fmt.Sprintf("echo json_encode(extension_loaded('%s'));", name))
	if err != nil {
		return false, err
	}

	var val bool
	if err := json.Unmarshal(outbuf.Bytes(), &val); err != nil {
		return false, err
	}

	return val, nil
}

func askAboutMissingArgs(u ui.UI, pkg peclpkg.Package, opts *BuildOpts) error {
	currentFlags := map[string]struct{}{}
	for _, flag := range opts.ConfigureArgs {
		segments := strings.SplitN(flag, "=", 2)
		flagName := strings.TrimLeft(segments[0], "-")
		currentFlags[flagName] = struct{}{}
	}

	for _, configOpt := range pkg.ExtSrcRelease.ConfigureOptions {
		if _, ok := currentFlags[configOpt.Name]; ok {
			continue
		}

		val, err := u.Prompt(configOpt.Prompt, configOpt.Default)
		if err != nil {
			return err
		}

		flag := "--" + configOpt.Name + "=" + val
		if strings.HasPrefix(configOpt.Name, "with-") && (val == "yes" || val == "autodetect") {
			flag = "--" + configOpt.Name
		}
		opts.ConfigureArgs = append(opts.ConfigureArgs, flag)
	}
	return nil
}
