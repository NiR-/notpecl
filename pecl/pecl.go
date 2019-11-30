package pecl

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"

	"github.com/NiR-/notpecl/ui"
	"github.com/mcuadros/go-version"
	"github.com/sirupsen/logrus"
	"golang.org/x/xerrors"
)

func NewPeclBackend(downloadDir, installDir string) (PeclBackend, error) {
	_, err := os.Stat(downloadDir)
	if os.IsNotExist(err) {
		return PeclBackend{}, xerrors.Errorf("download dir %q does not exist", downloadDir)
	}
	_, err = os.Stat(installDir)
	if os.IsNotExist(err) {
		return PeclBackend{}, xerrors.Errorf("install dir %q does not exist", installDir)
	}

	b := PeclBackend{
		ui:            ui.NewNonInteractiveUI(),
		downloadDir:   downloadDir,
		installDir:    installDir,
		peclBaseURI:   "https://pecl.php.net",
		extIndexURI:   "https://storage.googleapis.com/notpecl/extensions.json",
		httpTransport: &http.Transport{},
	}
	return b, nil
}

type PeclBackend struct {
	ui            ui.UI
	extIndex      map[string][]string
	downloadDir   string
	installDir    string
	peclBaseURI   string
	extIndexURI   string
	httpTransport *http.Transport
}

func (b PeclBackend) WithUI(ui ui.UI) PeclBackend {
	nb := b
	nb.ui = ui
	return nb
}

func (b PeclBackend) WithBaseURI(baseURI string) PeclBackend {
	nb := b
	nb.peclBaseURI = baseURI
	return nb
}

func (b PeclBackend) WithExtensionIndexURI(uri string) PeclBackend {
	nb := b
	nb.extIndexURI = uri
	return nb
}

func (b PeclBackend) WithHTTPTransport(t *http.Transport) PeclBackend {
	nb := b
	nb.httpTransport = t
	return nb
}

type InstallOpts struct {
	// Name is the name of the extension to install.
	Name string
	// Version is the exact version number of the extension to install.
	Version string
	// ConfigureArgs is a list of flags to pass to ./configure when building
	// the extension.
	ConfigureArgs []string
	// Parallel is the maximum number of parallel jobs executed by make at once.
	Parallel int
	// Clenaup indicated whether source code and build files should be removed
	// after sucessful builds.
	Cleanup bool
}

func (p PeclBackend) Install(ctx context.Context, opts InstallOpts) error {
	extDir, err := p.Download(ctx, opts.Name, opts.Version)
	if err != nil {
		return err
	}

	buildOpts := BuildOpts{
		ExtensionDir:   extDir,
		PackageXmlPath: path.Join(extDir, "package.xml"),
		ConfigureArgs:  opts.ConfigureArgs,
		Parallel:       opts.Parallel,
	}
	if err := p.Build(ctx, buildOpts); err != nil {
		return err
	}

	if opts.Cleanup {
		if err := os.RemoveAll(extDir); err != nil {
			return err
		}
	}

	return nil
}

func (b PeclBackend) ResolveConstraint(
	ctx context.Context,
	name,
	constraint string,
) (string, error) {
	if len(b.extIndex) == 0 {
		var err error
		b.extIndex, err = b.loadExtensionIndex()
		if err != nil {
			return "", err
		}
	}

	extVersions, ok := b.extIndex[name]
	if !ok {
		return "", xerrors.Errorf("could not find extension %q", name)
	}

	c := version.NewConstrainGroupFromString(constraint)
	for _, ver := range extVersions {
		if c.Match(ver) {
			return ver, nil
		}
	}

	return "", xerrors.Errorf("could not find a version of %q statisfying constraint %q", name, constraint)
}

func (b PeclBackend) loadExtensionIndex() (map[string][]string, error) {
	var index map[string][]string

	client := http.Client{Transport: b.httpTransport}
	resp, err := client.Get(b.extIndexURI)
	if err != nil {
		return index, xerrors.Errorf("could not download extension index: %v", err)
	}
	defer resp.Body.Close()

	raw, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return index, xerrors.Errorf("could not read extension index: %v", err)
	}

	if err := json.Unmarshal(raw, &index); err != nil {
		return index, xerrors.Errorf("could not unmarshal extension index: %v", err)
	}

	return index, nil
}

func (b PeclBackend) Download(
	ctx context.Context,
	name,
	version string,
) (string, error) {
	url := fmt.Sprintf("%s/get/%s-%s.tgz", b.peclBaseURI, name, version)
	fmt.Printf("Downloading %s...\n", url)

	// @TODO: check timeout and use context to cancel the request if needed
	client := http.Client{Transport: b.httpTransport}
	resp, err := client.Get(url)
	if err != nil {
		return "", xerrors.Errorf("could not download extension %q: %v", name, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", xerrors.Errorf("could not download extension %q: status code is not 200", name)
	}

	raw, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", xerrors.Errorf("could not download extension %q: %v", name, err)
	}

	rawbuf := bytes.NewBuffer(raw)

	contentLength, err := strconv.Atoi(resp.Header.Get("content-length"))
	if err != nil {
		return "", xerrors.Errorf("could not convert content-length to a number: %v", err)
	}
	if rawbuf.Len() != contentLength {
		return "", xerrors.Errorf("content length is %d, but only %d bytes read", rawbuf.Len(), contentLength)
	}

	rawr := bufio.NewReaderSize(rawbuf, contentLength)
	testBytes, err := rawr.Peek(64)
	if err != nil {
		return "", xerrors.Errorf("could not peek the two first bytes: %v", err)
	}

	contenttype := http.DetectContentType(testBytes)
	if contenttype != "application/x-gzip" {
		return "", xerrors.Errorf("could not download extension %q: content type is not suppored", name)
	}

	gzipr, err := gzip.NewReader(rawr)
	if err != nil {
		return "", xerrors.Errorf("could not uncompress %s.tar.gz: %v", name, err)
	}
	defer gzipr.Close()

	extDir := path.Join(b.downloadDir, name)
	dirPrefix := fmt.Sprintf("%s-%s/", name, version)
	tarr := tar.NewReader(gzipr)
	for {
		headers, err := tarr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}

		switch headers.Typeflag {
		case tar.TypeReg:
			if err := b.extractFile(extDir, dirPrefix, tarr, headers); err != nil {
				return "", err
			}
		}
	}

	return path.Join(b.downloadDir, name), nil
}

func (b PeclBackend) extractFile(
	extDir string,
	dirPrefix string,
	tarr *tar.Reader,
	headers *tar.Header,
) error {
	fullpath := path.Join(extDir, headers.Name)
	if strings.HasPrefix(headers.Name, dirPrefix) {
		trimmedPath := strings.TrimPrefix(headers.Name, dirPrefix)
		fullpath = path.Join(extDir, trimmedPath)
	}

	var filebuf bytes.Buffer
	var total int64

	for {
		raw := make([]byte, headers.Size-total)
		r, err := tarr.Read(raw)
		if err != nil && err != io.EOF {
			return err
		}

		filebuf.Write(raw[:r])
		total += int64(r)
		if err == io.EOF {
			break
		}
	}
	if total != headers.Size {
		return xerrors.Errorf("file %q is %d bytes long, but only %d read from tar archive", headers.Name, headers.Size, total)
	}

	os.MkdirAll(path.Dir(fullpath), 0750)

	logrus.Debugf("Unpacking %s (%d bytes)...\n", fullpath, headers.Size)
	if err := ioutil.WriteFile(fullpath, filebuf.Bytes(), 0640); err != nil {
		return err
	}

	return nil
}

type BuildOpts struct {
	// ExtensionDir is the folder containing extension source code.
	ExtensionDir string
	// PackageXmlPath is the full path to the package.xml for the extension to
	// build.
	PackageXmlPath string
	// ConfigureArgs is a list of flags to pass to ./configure when building.
	ConfigureArgs []string
	// Parallel is the maximum number of parallel jobs executed by make at once.
	Parallel int
}

// @TODO: fully implement package-2.0.xsd
func (b PeclBackend) Build(
	ctx context.Context,
	opts BuildOpts,
) error {
	var err error

	logrus.Debugf("Loading %s...", opts.PackageXmlPath)
	pkg, err := LoadPackageXMLFromFile(opts.PackageXmlPath)
	if err != nil {
		return xerrors.Errorf("failed to load package.xml: %v", err)
	}

	if err := checkPackageDependencies(pkg); err != nil {
		return err
	}
	if err := askAboutMissingArgs(b.ui, pkg, &opts); err != nil {
		return err
	}

	logrus.Debug("Running phpize...")
	err = runCommand(ctx, opts.ExtensionDir, "phpize")
	if err != nil {
		return xerrors.Errorf("failed to run phpize: %v", err)
	}

	phpConfigPath, _ := exec.LookPath("php-config")
	opts.ConfigureArgs = append(opts.ConfigureArgs, "--with-php-config="+phpConfigPath)

	logrus.Debug("Running ./configure...")
	configurePath := path.Join(opts.ExtensionDir, "configure")
	err = runCommand(ctx, opts.ExtensionDir, configurePath, opts.ConfigureArgs...)
	if err != nil {
		return xerrors.Errorf("failed to run configure: %v", err)
	}

	logrus.Debug("Running make...")
	err = runCommand(ctx, opts.ExtensionDir, "make")
	if err != nil {
		return xerrors.Errorf("failed to run make: %v", err)
	}

	logrus.Debug("Running make install...")
	args := []string{
		"-j", strconv.Itoa(int(opts.Parallel)),
		"INSTALL_ROOT=" + b.installDir,
		"install",
	}
	err = runCommand(ctx, opts.ExtensionDir, "make", args...)
	if err != nil {
		return xerrors.Errorf("failed to run make install: %v", err)
	}

	logrus.Debug("Running make clean...")
	err = runCommand(ctx, opts.ExtensionDir, "make", "clean")
	if err != nil {
		return xerrors.Errorf("failed to run make clean: %v", err)
	}

	return nil
}

var (
	defaultCflags   = "-fstack-protector-strong -fpic -fpie -O2 -D_LARGEFILE_SOURCE -D_FILE_OFFSET_BITS=64"
	defaultCppflags = "-fstack-protector-strong -fpic -fpie -O2 -D_LARGEFILE_SOURCE -D_FILE_OFFSET_BITS=64"
	defaultLdflags  = "-Wl,-O1 -Wl,--hash-style=both -pie"
)

func runCommand(
	ctx context.Context,
	dir string,
	cmd string,
	args ...string,
) error {
	c := exec.CommandContext(ctx, cmd, args...)
	c.Dir = dir
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	c.Env = []string{
		"PATH=" + lookupEnv("PATH", ""),
		"CFLAGS=" + lookupEnv("PHP_CFLAGS", defaultCflags),
		"CPPFLAGS=" + lookupEnv("PHP_CPPFLAGS", defaultCppflags),
		"LDFLAGS=" + lookupEnv("PHP_LDFLAGS", defaultLdflags),
	}
	return c.Run()
}

var envCache = map[string]string{}

func lookupEnv(name, defaultVal string) string {
	val, ok := envCache[name]
	if !ok {
		val, _ = os.LookupEnv(name)
		envCache[name] = val
	}
	if val == "" {
		val = defaultVal
	}
	return val
}

func checkPackageDependencies(pkg Package) error {
	logrus.Debug("Checking extension dependencies...")
	if err := checkPHPVersion(pkg.Dependencies.Required.PHP); err != nil {
		return err
	}

	for _, dep := range pkg.Dependencies.Required.Extensions {
		isEnabled, err := isExtensionEnabled(dep.Name)
		if err != nil {
			return err
		}
		if !isEnabled {
			return xerrors.Errorf("Extension %q is required but is not enabled.", dep.Name)
		}
	}

	for _, dep := range pkg.Dependencies.Optional.Extensions {
		isEnabled, err := isExtensionEnabled(dep.Name)
		if err != nil {
			return err
		}
		if !isEnabled {
			logrus.Infof("Optional extension %q is not enabled.", dep.Name)
		}
	}

	return nil
}

func checkPHPVersion(extConstraint PHPConstraint) error {
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

	currentVersion, err := currentPHPVersion()
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

func currentPHPVersion() (string, error) {
	var outbuf bytes.Buffer
	cmd := exec.Command("php", "-r", "echo json_encode(PHP_VERSION);")
	cmd.Stdout = &outbuf

	if err := cmd.Run(); err != nil {
		return "", err
	}

	var phpVersion string
	if err := json.Unmarshal(outbuf.Bytes(), &phpVersion); err != nil {
		return "", err
	}

	return phpVersion, nil
}

func isExtensionEnabled(name string) (bool, error) {
	var outbuf bytes.Buffer
	cmd := exec.Command(
		"php", "-r",
		fmt.Sprintf("echo json_encode(extension_loaded('%s'));", name))
	cmd.Stdout = &outbuf
	if err := cmd.Run(); err != nil {
		return false, err
	}

	var val bool
	if err := json.Unmarshal(outbuf.Bytes(), &val); err != nil {
		return false, err
	}

	return val, nil
}

func askAboutMissingArgs(u ui.UI, pkg Package, opts *BuildOpts) error {
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
