package pecl_test

import (
	"context"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"path"
	"strconv"
	"testing"
	"time"

	"github.com/NiR-/notpecl/pecl"
	"github.com/tv42/httpunix"
	"golang.org/x/xerrors"
)

type peclDownloadTC struct {
	backend     pecl.PeclBackend
	downloadDir string
	expectedErr error
	cleanup     func()
}

func initDownloadZip1155TC(
	t *testing.T,
	httpT *http.Transport,
	baseURI string,
) peclDownloadTC {
	cacheDir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatal(err)
	}

	downloadDir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatal(err)
	}

	cleanup := func() {
		if err := os.RemoveAll(cacheDir); err != nil {
			t.Fatal(err)
		}
		if err := os.RemoveAll(downloadDir); err != nil {
			t.Fatal(err)
		}
	}

	b, err := pecl.NewPeclBackend(cacheDir, downloadDir, downloadDir)
	if err != nil {
		t.Fatal(err)
	}

	b = b.WithHTTPTransport(httpT)
	b = b.WithBaseURI(baseURI)

	return peclDownloadTC{
		backend:     b,
		downloadDir: downloadDir,
		cleanup:     cleanup,
	}
}

func TestPeclDownload(t *testing.T) {
	testcases := map[string]func(t *testing.T, httpT *http.Transport, baseURI string) peclDownloadTC{
		"successfully download zip-1.15.5": initDownloadZip1155TC,
	}

	for tcname := range testcases {
		tcinit := testcases[tcname]

		t.Run(tcname, func(t *testing.T) {
			t.Parallel()

			socketPath, srvStop := startPeclServer(
				t,
				"/get/zip-1.15.5.tgz",
				serveExtTgzHandler(t, "testdata/pecl.php.net/zip-1.15.5.tgz"),
			)
			defer srvStop()

			ut := &httpunix.Transport{}
			ut.RegisterLocation("notpecl", socketPath)
			httpT := &http.Transport{}
			httpT.RegisterProtocol(httpunix.Scheme, ut)

			tc := tcinit(t, httpT, "http+unix://notpecl")
			defer tc.cleanup()

			ctx := context.TODO()
			_, err := tc.backend.Download(ctx, "zip", "1.15.5")
			if tc.expectedErr != nil {
				if err == nil {
					t.Fatalf("Expected error: %v\nGot: <nil>", tc.expectedErr)
				}
				if tc.expectedErr.Error() != err.Error() {
					t.Fatalf("Expected error: %v\nGot: %v", tc.expectedErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			extPath := path.Join(tc.downloadDir, "zip")
			_, err = os.Stat(extPath)
			if os.IsNotExist(err) {
				t.Fatalf("Expected %q to exist, but does not.", extPath)
			}
			if err != nil {
				t.Fatal(err)
			}

			expected, err := ioutil.ReadFile("testdata/package-zip.xml")
			if err != nil {
				t.Fatal(err)
			}

			xmlpath := path.Join(extPath, "package.xml")
			downloadedXml, err := ioutil.ReadFile(xmlpath)
			if err != nil {
				t.Fatal(err)
			}

			if string(expected) != string(downloadedXml) {
				t.Fatalf("Expected: %v\nGot: %v", string(expected), string(downloadedXml))
			}
		})
	}
}

type peclInstallTC struct {
	backend       pecl.PeclBackend
	configureArgs []string
	expectedErr   error
	cleanup       func()
}

func initInstallZip1155TC(
	t *testing.T,
	httpT *http.Transport,
	baseURI string,
) peclInstallTC {
	cacheDir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatal(err)
	}

	downloadDir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatal(err)
	}

	installDir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatal(err)
	}

	cleanup := func() {
		if err := os.RemoveAll(cacheDir); err != nil {
			t.Fatal(err)
		}
		if err := os.RemoveAll(downloadDir); err != nil {
			t.Fatal(err)
		}
		if err := os.RemoveAll(installDir); err != nil {
			t.Fatal(err)
		}
	}

	b, err := pecl.NewPeclBackend(cacheDir, downloadDir, downloadDir)
	if err != nil {
		t.Fatal(err)
	}

	mockUI := mockInteractiveUI{
		questions: map[string]string{
			"enable igbinary serializer support?": "no",
			"enable zstd compression support?":    "yes",
		},
	}

	b = b.WithHTTPTransport(httpT)
	b = b.WithBaseURI(baseURI)
	b = b.WithUI(mockUI)

	return peclInstallTC{
		backend:       b,
		configureArgs: []string{"--enable-redis-lzf"},
		cleanup:       cleanup,
	}
}

type mockInteractiveUI struct {
	questions map[string]string
}

func (m mockInteractiveUI) Prompt(question, defaultVal string) (string, error) {
	val, ok := m.questions[question]
	if !ok {
		return "", xerrors.Errorf("question %q is not expected", question)
	}
	return val, nil
}

func TestPeclInstall(t *testing.T) {
	testcases := map[string]func(t *testing.T, httpT *http.Transport, baseURI string) peclInstallTC{
		"successfully install redis-5.1.1": initInstallZip1155TC,
	}

	for tcname := range testcases {
		tcinit := testcases[tcname]

		t.Run(tcname, func(t *testing.T) {
			t.Parallel()

			socketPath, srvStop := startPeclServer(
				t,
				"/get/redis-5.1.1.tgz",
				serveExtTgzHandler(t, "testdata/pecl.php.net/redis-5.1.1.tgz"),
			)
			defer srvStop()

			ut := &httpunix.Transport{}
			ut.RegisterLocation("notpecl", socketPath)
			httpT := &http.Transport{}
			httpT.RegisterProtocol(httpunix.Scheme, ut)

			tc := tcinit(t, httpT, "http+unix://notpecl")
			defer tc.cleanup()

			ctx := context.TODO()
			opts := pecl.InstallOpts{
				Name:          "redis",
				Version:       "5.1.1",
				ConfigureArgs: tc.configureArgs,
				Parallel:      1,
			}
			err := tc.backend.Install(ctx, opts)
			if tc.expectedErr != nil {
				if err == nil {
					t.Fatalf("Expected error: %v\nGot: <nil>", tc.expectedErr)
				}
				if tc.expectedErr.Error() != err.Error() {
					t.Fatalf("Expected error: %v\nGot: %v", tc.expectedErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
		})
	}
}

func startPeclServer(
	t *testing.T,
	route string,
	handler func(http.ResponseWriter, *http.Request),
) (string, func()) {
	mux := http.NewServeMux()
	mux.HandleFunc(route, handler)

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

func serveExtTgzHandler(t *testing.T, fullpath string) func(http.ResponseWriter, *http.Request) {
	buf, err := ioutil.ReadFile(fullpath)
	if err != nil {
		t.Fatal(err)
	}

	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Length", strconv.Itoa(len(buf)))
		w.WriteHeader(200)
		w.Write(buf)
	}
}
