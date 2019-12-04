package extindex_test

import (
	"net"
	"net/http"
	"os"
	"path"
	"strconv"
	"testing"
	"time"

	"github.com/NiR-/notpecl/extindex"
	"github.com/go-test/deep"
	"github.com/tv42/httpunix"
	"golang.org/x/xerrors"
)

func TestLoadExtensionIndex(t *testing.T) {
	testcases := map[string]struct {
		statusCode  int
		expected    extindex.ExtIndex
		expectedErr error
	}{
		"successfully resolve redis version": {
			statusCode: 200,
			expected: extindex.ExtIndex{
				"AOP": extindex.ExtVersions{
					"0.1.0":   extindex.Beta,
					"0.2.2b1": extindex.Beta,
				},
				"APCu": extindex.ExtVersions{
					"4.0.0": extindex.Beta,
					"4.0.2": extindex.Beta,
					"4.0.3": extindex.Beta,
					"5.1.8": extindex.Stable,
					"5.1.9": extindex.Stable,
				},
			},
		},
		"fail to load extension index when http response is not 200": {
			statusCode:  500,
			expectedErr: xerrors.New("could not download extension index: status code is 500"),
		},
	}

	for tcname := range testcases {
		tc := testcases[tcname]

		t.Run(tcname, func(t *testing.T) {
			t.Parallel()

			h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tc.statusCode != 200 {
					w.WriteHeader(tc.statusCode)
					return
				}
				http.ServeFile(w, r, "testdata/extensions.json")
			})
			socketPath, srvStop := startHTTPServer(t, "/extensions.json", h)
			defer srvStop()

			unixT := &httpunix.Transport{}
			unixT.RegisterLocation("notpecl", socketPath)
			httpT := &http.Transport{}
			httpT.RegisterProtocol(httpunix.Scheme, unixT)

			index, err := extindex.LoadExtensionIndex(extindex.LoadOpts{
				HttpTransport: httpT,
				ExtIndexURI:   "http+unix://notpecl/extensions.json",
			})
			if tc.expectedErr != nil {
				if err == nil || tc.expectedErr.Error() != err.Error() {
					t.Fatalf("Expected error: %v\nGot: %v", tc.expectedErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if diff := deep.Equal(index, tc.expected); diff != nil {
				t.Fatal(diff)
			}
		})
	}
}

func TestExtIndexSort(t *testing.T) {
	testcases := map[string]struct {
		original extindex.ExtVersions
		expected []string
	}{
		"successfully sort versions": {
			original: extindex.ExtVersions{
				"1.3.0": extindex.Stable,
				"1.5.3": extindex.Stable,
				"1.1.1": extindex.Stable,
				"2.1.4": extindex.Stable,
			},
			expected: []string{"2.1.4", "1.5.3", "1.3.0", "1.1.1"},
		},
	}

	for tcname := range testcases {
		tc := testcases[tcname]
		t.Run(tcname, func(t *testing.T) {
			sorted := tc.original.Sort()
			if diff := deep.Equal(sorted, tc.expected); diff != nil {
				t.Fatal(diff)
			}
		})
	}
}

func startHTTPServer(
	t *testing.T,
	route string,
	handler http.Handler,
) (string, func()) {
	mux := http.NewServeMux()
	mux.Handle(route, handler)

	socketPath := path.Join(os.TempDir(), random()+".sock")
	unixListener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatal(err)
	}

	srv := &http.Server{
		Handler: mux,
	}
	go srv.Serve(unixListener)

	srvStop := func() {
		srv.Close()
		unixListener.Close()
		os.Remove(socketPath)
	}

	return socketPath, srvStop
}

// Inspired by https://github.com/golang/go/blob/master/src/io/ioutil/tempfile.go#L27
func random() string {
	r := uint32(time.Now().UnixNano())
	r = r*1664525 + 1013904223 // constants from Numerical Recipes
	return strconv.Itoa(int(1e9 + r%1e9))[1:]
}
