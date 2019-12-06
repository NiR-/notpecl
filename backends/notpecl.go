package backends

import (
	"context"
	"net/http"

	"github.com/NiR-/notpecl/extindex"
	"github.com/mcuadros/go-version"
	"golang.org/x/xerrors"
)

func NewNotPeclBackend() NotPeclBackend {
	return NotPeclBackend{}
}

type NotPeclBackend struct {
	extIndex extindex.ExtIndex
}

func (b NotPeclBackend) WithExtensionIndex(index extindex.ExtIndex) NotPeclBackend {
	nb := b
	nb.extIndex = index
	return nb
}

func (b NotPeclBackend) ResolveConstraint(
	ctx context.Context,
	name,
	constraint string,
	minimumStability extindex.Stability,
) (string, error) {
	if len(b.extIndex) == 0 {
		var err error
		b.extIndex, err = extindex.LoadExtensionIndex(extindex.LoadOpts{
			HttpTransport: &http.Transport{},
			ExtIndexURI:   "https://storage.googleapis.com/notpecl/extensions.json",
		})
		if err != nil {
			return "", err
		}
	}

	extVersions, ok := b.extIndex[name]
	if !ok {
		return "", xerrors.Errorf("could not find extension %q", name)
	}

	c := version.NewConstrainGroupFromString(constraint)
	sortedVersions := extVersions.Sort()

	for i := 0; i < len(sortedVersions); i++ {
		extVer := sortedVersions[i]
		stability := extVersions[extVer]
		if stability < minimumStability {
			continue
		}
		if c.Match(extVer) {
			return extVer, nil
		}
	}

	return "", xerrors.Errorf("could not find a version of %s satisfying %q", name, constraint)
}
