package backends_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/NiR-/notpecl/backends"
	"github.com/NiR-/notpecl/extindex"
	"github.com/tv42/httpunix"
	"golang.org/x/xerrors"
)

func TestPeclResolveConstraint(t *testing.T) {
	testcases := map[string]struct {
		extIndex    extindex.ExtIndex
		name        string
		constraint  string
		expected    string
		expectedErr error
	}{
		"successfully resolve redis version": {
			extIndex: extindex.ExtIndex{
				"redis": extindex.ExtVersions{
					"5.1.1": extindex.Stable,
					"5.1.0": extindex.Stable,
				},
			},
			name:       "redis",
			constraint: "~5.1.0",
			expected:   "5.1.1",
		},
		"fail to resolve version with lower stability": {
			extIndex: extindex.ExtIndex{
				"redis": extindex.ExtVersions{
					"5.1.1": extindex.Beta,
					"5.1.0": extindex.Alpha,
				},
			},
			name:        "redis",
			constraint:  "~5.1.0",
			expectedErr: xerrors.New("could not find a version of redis satisfying \"~5.1.0\""),
		},
		"fail to resolve unknown extension": {
			extIndex: extindex.ExtIndex{
				"redis": extindex.ExtVersions{},
			},
			name:        "unknownext",
			constraint:  ">=1.2.3",
			expectedErr: xerrors.New("could not find extension \"unknownext\""),
		},
	}

	for tcname := range testcases {
		tc := testcases[tcname]

		t.Run(tcname, func(t *testing.T) {
			t.Parallel()

			h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.ServeFile(w, r, "testdata/extensions.json")
			})
			socketPath, srvStop := startHTTPServer(t, "/extensions.json", h)
			defer srvStop()

			unixT := &httpunix.Transport{}
			unixT.RegisterLocation("notpecl", socketPath)
			httpT := &http.Transport{}
			httpT.RegisterProtocol(httpunix.Scheme, unixT)

			b := backends.NewNotPeclBackend()
			b = b.WithExtensionIndex(tc.extIndex)

			ctx := context.Background()
			resolved, err := b.ResolveConstraint(ctx, tc.name, tc.constraint, extindex.Stable)
			if tc.expectedErr != nil {
				if err == nil || tc.expectedErr.Error() != err.Error() {
					t.Fatalf("Expected error: %v\nGot: %v", tc.expectedErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if resolved != tc.expected {
				t.Fatalf("Expected: %s\nGot: %s", tc.expected, resolved)
			}
		})
	}
}
