package peclpkg

import (
	"bytes"
	"encoding/xml"
	"io"
	"io/ioutil"

	"github.com/sirupsen/logrus"
	"golang.org/x/text/encoding/ianaindex"
	"golang.org/x/text/transform"
)

// LoadPackageXMLFromFile loads the XML file at xmlpath and converts it into a
// Package struct.
func LoadPackageXMLFromFile(xmlpath string) (Package, error) {
	raw, err := ioutil.ReadFile(xmlpath)
	if err != nil {
		return Package{}, err
	}

	rawr := bytes.NewBuffer(raw)
	return LoadPackageXML(rawr)
}

// LoadPackageXML reads an XML file from the given io.Reader and transforms it
// into a Package struct.
func LoadPackageXML(r io.Reader) (Package, error) {
	decoder := xml.NewDecoder(r)
	decoder.CharsetReader = charsetReader

	var pkg Package
	if err := decoder.Decode(&pkg); err != nil {
		return pkg, err
	}

	return pkg, nil
}

func charsetReader(charset string, input io.Reader) (io.Reader, error) {
	enc, err := ianaindex.IANA.Encoding(charset)
	if err != nil {
		return nil, err
	}
	return transform.NewReader(input, enc.NewDecoder()), nil
}

// Package represents a package.xml file.
type Package struct {
	Name          string           `xml:"name"`
	Summary       string           `xml:"summary"`
	Description   string           `xml:"description"`
	PublishDate   string           `xml:"date"`
	PublishTime   string           `xml:"time"`
	User          string           `xml:"user"`
	Email         string           `xml:"email"`
	Version       Version          `xml:"version"`
	Stability     PackageStability `xml:"stability"`
	License       License          `xml:"license"`
	Dependencies  Dependencies     `xml:"dependencies"`
	ExtSrcRelease ExtSrcRelease    `xml:"extsrcrelease"`
	Changelog     Changelog        `xml:"changelog"`
}

// Changelog contains all the releases of a specific Package.
type Changelog struct {
	Releases []Release `xml:"release"`
}

// Release contains all the metadata about a single release as provided by
// package.xml files.
type Release struct {
	Date      string           `xml:"date"`
	Time      string           `xml:"time"`
	Version   Version          `xml:"version"`
	Stability PackageStability `xml:"stability"`
	Notes     string           `xml:"notes"`
}

type Version struct {
	Release string `xml:"release"`
	API     string `xml:"api"`
}

type PackageStability struct {
	Release Stability `xml:"release"`
	API     Stability `xml:"api"`
}

type Stability string

func (s *Stability) UnmarshalText(text []byte) error {
	switch string(text) {
	case string(Snapshot):
		*s = Snapshot
	case string(Devel):
		*s = Devel
	case string(Alpha):
		*s = Alpha
	case string(Beta):
		*s = Beta
	case string(Stable):
		*s = Stable
	default:
		logrus.Warnf("unsupported stability %q", string(text))
	}
	return nil
}

var (
	Snapshot Stability = "snapshot"
	Devel    Stability = "devel"
	Alpha    Stability = "alpha"
	Beta     Stability = "beta"
	Stable   Stability = "stable"
)

type License struct {
	Name string `xml:",chardata"`
	URI  string `xml:"uri,attr"`
}

type Dependencies struct {
	Required RequiredDependencies `xml:"required"`
	Optional OptionalDependencies `xml:"optional"`
}

type RequiredDependencies struct {
	PHP        PHPConstraint         `xml:"php"`
	Extensions []ExtensionConstraint `xml:"extension"`
}

type OptionalDependencies struct {
	Extensions []ExtensionConstraint `xml:"extension"`
}

type PHPConstraint struct {
	Min     string   `xml:"min"`
	Max     string   `xml:"max"`
	Exclude []string `xml:"exclude"`
}

type ExtensionConstraint struct {
	Name    string   `xml:"name"`
	Min     string   `xml:"min"`
	Max     string   `xml:"max"`
	Exclude []string `xml:"exclude"`
}

type ExtSrcRelease struct {
	ConfigureOptions []ConfigureOption `xml:"configureoption"`
}

type ConfigureOption struct {
	Name    string `xml:"name,attr"`
	Default string `xml:"default,attr"`
	Prompt  string `xml:"prompt,attr"`
}
