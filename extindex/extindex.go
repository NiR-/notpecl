package extindex

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"sort"

	"github.com/mcuadros/go-version"
	"golang.org/x/xerrors"
)

type LoadOpts struct {
	HttpTransport *http.Transport
	ExtIndexURI   string
}

func LoadExtensionIndex(opts LoadOpts) (ExtIndex, error) {
	var index ExtIndex

	client := http.Client{Transport: opts.HttpTransport}
	resp, err := client.Get(opts.ExtIndexURI)
	if err != nil {
		return index, xerrors.Errorf("could not download extension index: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return index, xerrors.Errorf("could not download extension index: status code is %d", resp.StatusCode)
	}

	raw, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return index, xerrors.Errorf("could not read extension index: %v", err)
	}

	if err := json.Unmarshal(raw, &index); err != nil {
		return index, xerrors.Errorf("could not unmarshal extension index: %v", err)
	}

	return index, nil
}

// ExtIndex is the list of extensions associated to their versions/stability
type ExtIndex map[string]ExtVersions

// ExtVersions is the list of versions associated to their stability, for a given extension
type ExtVersions map[string]Stability

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

func (s *Stability) UnmarshalJSON(raw []byte) error {
	var str string
	if err := json.Unmarshal(raw, &str); err != nil {
		return err
	}
	*s = StabilityFromString(str)
	return nil
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

// Sort returns a slice containing the versions of the extension sorted in
// descending order.
func (ev ExtVersions) Sort() []string {
	toSort := make([]string, 0, len(ev))
	for extVersion := range ev {
		toSort = append(toSort, extVersion)
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
