// peclapi package implements a HTTP-based client for the REST API initially
// used by pecl. It only supports the default pecl channel served at
// https://pecl.php.net/rest/.
// Note that this API is quite dated and might return some unexpected results
// (eg. redis extension is in the Database category but ListPackagesInCategory("Database")
// won't return redis).
package peclapi

import (
	"bufio"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"sort"

	"github.com/mcuadros/go-version"
	"golang.org/x/text/encoding/ianaindex"
	"golang.org/x/text/transform"
	"golang.org/x/xerrors"
)

// NewClient creates a new HTTP-based client for the API hosted at
// pecl.php.net/redis/. It takes ClientOpt as arguments. These could be used to
// set Client's internal properties (baseURI or httpClient).
func NewClient(opts ...ClientOpt) Client {
	c := Client{
		baseURI:    "https://pecl.php.net/rest",
		httpClient: &http.Client{},
	}
	for _, opt := range opts {
		opt(&c)
	}
	return c
}

// ClientOpt are functions used by NewClient() to set Clients' internal
// properties.
type ClientOpt func(*Client)

// Client represents the HTTP-based client for https://pecl.php.net/rest/.
type Client struct {
	baseURI    string
	httpClient *http.Client
}

// WithBaseURI returns a ClientOpt that could be passed to NewClient to set the
// API base URI.
func WithBaseURI(baseURI string) ClientOpt {
	return func(c *Client) {
		c.baseURI = baseURI
	}
}

// WithHttpClient returns a ClientOpt that could be passed to NewClient to set
// the HTTP client used to query the API.
func WithHttpClient(httpClient *http.Client) ClientOpt {
	return func(c *Client) {
		c.httpClient = httpClient
	}
}

type packageList struct {
	Packages []string `xml:"p"`
}

// ListPackages returns the list of packages available at the endpoint
// /p/packages.xml. It returns an error if the HTTP request fails or if the
// endpoint returns a non-200 status code.
func (c Client) ListPackages() ([]string, error) {
	url := fmt.Sprintf("%s/p/packages.xml", c.baseURI)
	resp, err := c.httpClient.Get(url)
	if err != nil {
		return []string{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return []string{}, xerrors.Errorf("could not list packages: expected status code 200, got %d", resp.StatusCode)
	}

	decoder := xml.NewDecoder(resp.Body)
	decoder.CharsetReader = charsetReader

	var list packageList
	if err := decoder.Decode(&list); err != nil {
		return []string{}, xerrors.Errorf("could not list packages: %w", err)
	}

	return list.Packages, nil
}

// ListPackagesInCategory returns the list of packages in the given category,
// as served by the endpoint /c/{category}/package.xml. It returns an error if
// the request fails or if th endpoint returns a non-200 status code.
func (c Client) ListPackagesInCategory(category string) ([]string, error) {
	url := fmt.Sprintf("%s/c/%s/packages.xml", c.baseURI, category)
	resp, err := c.httpClient.Get(url)
	if err != nil {
		return []string{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return []string{}, xerrors.Errorf("could not list packages in %s category: category not found", category)
	}
	if resp.StatusCode != 200 {
		return []string{}, xerrors.Errorf("could not list packages in %s category: expected status code 200, got %d", category, resp.StatusCode)
	}

	decoder := xml.NewDecoder(resp.Body)
	decoder.CharsetReader = charsetReader

	var list packageList
	if err := decoder.Decode(&list); err != nil {
		return []string{}, xerrors.Errorf("could not list packages in %s category: %w", category, err)
	}

	return list.Packages, nil
}

// Package represents a detailed package as returned by DescribePackage().
// See https://pear.php.net/dtd/rest.package.xsd.
type Package struct {
	Name               string `xml:"n"`
	Category           string `xml:"ca"`
	License            string `xml:"l"`
	Summary            string `xml:"s"`
	Description        string `xml:"d"`
	ReleasesLocation   string `xml:"r"`
	ParentPackage      string `xml:"pa"`
	DeprecatingPackage string `xml:"dp"`
	DeprecatingChannel string `xml:"dc"`
}

// DescribePackage returns the details of a given package as served by the
// endpoint /p/{packageName}/info.xml. It returns an error if the request
// fails or if the endpoint returns a non-200 status code.
func (c Client) DescribePackage(pkgName string) (Package, error) {
	var pkg Package

	url := fmt.Sprintf("%s/p/%s/info.xml", c.baseURI, pkgName)
	resp, err := c.httpClient.Get(url)
	if err != nil {
		return pkg, xerrors.Errorf("could not describe package %s: %w", pkgName, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return pkg, xerrors.Errorf("could not describe package %s: package not found", pkgName)
	}
	if resp.StatusCode != 200 {
		return pkg, xerrors.Errorf("could not describe package %s: expected status code 200, got %d", pkgName, resp.StatusCode)
	}

	decoder := xml.NewDecoder(resp.Body)
	decoder.CharsetReader = charsetReader

	if err := decoder.Decode(&pkg); err != nil {
		return pkg, xerrors.Errorf("could not describe package %s: %w", pkgName, err)
	}

	return pkg, nil
}

type releaseList struct {
	Releases []release `xml:"r"`
}

type release struct {
	Version   string `xml:"v"`
	Stability string `xml:"s"`
}

// PackageReleases is the list of releases associated to their stabilit for a
// given package. It supports sorting.
type PackageReleases map[string]Stability

type Stability int

func StabilityFromString(s string) Stability {
	switch string(s) {
	case "snapshot":
		return Snapshot
	case "devel":
		return Devel
	case "alpha":
		return Alpha
	case "beta":
		return Beta
	case "stable":
		return Stable
	default:
		return Unknown
	}
}

func (s Stability) String() string {
	switch s {
	case Snapshot:
		return "snapshot"
	case Devel:
		return "devel"
	case Alpha:
		return "alpha"
	case Beta:
		return "beta"
	case Stable:
		return "stable"
	default:
		return "unknown"
	}
}

const (
	Unknown Stability = iota
	Snapshot
	Devel
	Alpha
	Beta
	Stable
)

// Sort returns a slice containing the releases of the package sorted in
// descending order.
func (pr PackageReleases) Sort() []string {
	toSort := make([]string, 0, len(pr))
	for release := range pr {
		toSort = append(toSort, release)
	}

	sort.Sort(sort.Reverse(versionSlice(toSort)))
	return toSort
}

type versionSlice []string

func (s versionSlice) Len() int {
	return len(s)
}

func (s versionSlice) Less(i, j int) bool {
	return version.Compare(s[i], s[j], "<")
}

func (s versionSlice) Swap(i, j int) {
	vi := s[i]
	vj := s[j]
	s[i] = vj
	s[j] = vi
}

// ListReleases returns the list of releases associated with their stability
// for a given package, as served by the endpoint /r/{packageName}/allreleases.xml.
// It returns an error if the request fails or if the endpoint returns a
// non-200 status code.
func (c Client) ListReleases(pkgName string) (PackageReleases, error) {
	releases := make(PackageReleases)

	url := fmt.Sprintf("%s/r/%s/allreleases.xml", c.baseURI, pkgName)
	resp, err := c.httpClient.Get(url)
	if err != nil {
		return releases, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return releases, xerrors.Errorf("could not list releases for %s: package not found", pkgName)
	}
	if resp.StatusCode != 200 {
		return releases, xerrors.Errorf("could not list releases for %s: expected status code 200, got %d", pkgName, resp.StatusCode)
	}

	decoder := xml.NewDecoder(resp.Body)
	decoder.CharsetReader = charsetReader

	var list releaseList
	if err := decoder.Decode(&list); err != nil {
		return releases, xerrors.Errorf("could not list releases for %s: %w", pkgName, err)
	}

	for _, r := range list.Releases {
		releases[r.Version] = StabilityFromString(r.Stability)
	}
	return releases, nil
}

// DescribeRelease returns the details about a given release of a given
// package, as served by the endpoint /r/{packageName}/{release}.xml. It
// returns an error if the request fails or if the endpoint returns a
// non-200 status code.
func (c Client) DescribeRelease(pkgName, pkgVersion string) (Release, error) {
	var release Release

	url := fmt.Sprintf("%s/r/%s/%s.xml", c.baseURI, pkgName, pkgVersion)
	resp, err := c.httpClient.Get(url)
	if err != nil {
		return release, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return release, xerrors.Errorf("could not describe %s release %s: release not found", pkgName, pkgVersion)
	}
	if resp.StatusCode != 200 {
		return release, xerrors.Errorf("could not describe %s releases %s: expected status code 200, got %d", pkgName, pkgVersion, resp.StatusCode)
	}

	decoder := xml.NewDecoder(resp.Body)
	decoder.CharsetReader = charsetReader

	if err := decoder.Decode(&release); err != nil {
		return release, xerrors.Errorf("could not describe %s release %s: %w", pkgName, pkgVersion, err)
	}

	return release, nil
}

// Release represents the details of a specific release of a package.
// See https://pear.php.net/dtd/rest.release.xsd.
type Release struct {
	Package      string `xml:"p"`
	Version      string `xml:"v"`
	Stability    string `xml:"st"`
	License      string `xml:"l"`
	Maintainer   string `xml:"m"`
	Summary      string `xml:"s"`
	Description  string `xml:"d"`
	ReleaseDate  string `xml:"da"`
	ReleaseNotes string `xml:"n"`
	PartialURI   string `xml:"g"`
	PackageXML   string `xml:"x"`
}

// DownloadRelease downloads a given package release and returns an
// io.ReadCloser from which a tar can be read. An error is returned if the HTTP
// request fails, if a bad status code is returned or if the downloaded file is
// not an application/x-gzip.
func (c Client) DownloadRelease(release Release) (io.Reader, error) {
	if release.PartialURI == "" {
		return nil, xerrors.Errorf("empty PartialURI")
	}

	url := fmt.Sprintf("%s.tgz", release.PartialURI)
	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, xerrors.Errorf("could not download %s v%s: %w", release.Package, release.Version, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, xerrors.Errorf("could not download %s v%s: expected status code 200, got %d", release.Package, release.Version, resp.StatusCode)
	}

	rawr := bufio.NewReaderSize(resp.Body, int(resp.ContentLength))
	testBytes, err := rawr.Peek(64)
	if err != nil {
		return nil, xerrors.Errorf("could not peek the first 64 bytes of the downloaded file: %w", err)
	}

	contentType := http.DetectContentType(testBytes)
	if contentType != "application/x-gzip" {
		return nil, xerrors.Errorf("the file downloaded at %s isn't a gzip file", url)
	}

	return rawr, nil
}

func charsetReader(charset string, input io.Reader) (io.Reader, error) {
	enc, err := ianaindex.IANA.Encoding(charset)
	if err != nil {
		return nil, err
	}
	return transform.NewReader(input, enc.NewDecoder()), nil
}
