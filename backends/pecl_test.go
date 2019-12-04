package backends_test

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

	"github.com/NiR-/notpecl/backends"
	"github.com/tv42/httpunix"
	"golang.org/x/xerrors"
)

type peclDownloadTC struct {
	backend     backends.PeclBackend
	handler     http.Handler
	downloadDir string
	expectedErr error
	cleanup     func()
}

func initDownloadZip1155TC(t *testing.T) peclDownloadTC {
	downloadDir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatal(err)
	}

	cleanup := func() {
		if err := os.RemoveAll(downloadDir); err != nil {
			t.Fatal(err)
		}
	}

	np := backends.NewNotPeclBackend()
	b, err := backends.NewPeclBackend(np, downloadDir, downloadDir)
	if err != nil {
		t.Fatal(err)
	}

	return peclDownloadTC{
		backend: b,
		handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.ServeFile(w, r, "testdata/pecl.php.net/zip-1.15.5.tgz")
		}),
		downloadDir: downloadDir,
		cleanup:     cleanup,
	}
}

func initFailDownloadWithBadStatusCodeTC(t *testing.T) peclDownloadTC {
	downloadDir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatal(err)
	}

	cleanup := func() {
		if err := os.RemoveAll(downloadDir); err != nil {
			t.Fatal(err)
		}
	}

	np := backends.NewNotPeclBackend()
	b, err := backends.NewPeclBackend(np, downloadDir, downloadDir)
	if err != nil {
		t.Fatal(err)
	}

	return peclDownloadTC{
		backend: b,
		handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(500)
		}),
		downloadDir: downloadDir,
		cleanup:     cleanup,
		expectedErr: xerrors.New("could not download zip extension: expected status code 200, got 500"),
	}
}

func TestPeclDownload(t *testing.T) {
	testcases := map[string]func(t *testing.T) peclDownloadTC{
		"successfully download zip-1.15.5":                           initDownloadZip1155TC,
		"installation fail when the download status code is not 200": initFailDownloadWithBadStatusCodeTC,
	}

	for tcname := range testcases {
		tcinit := testcases[tcname]

		t.Run(tcname, func(t *testing.T) {
			t.Parallel()

			tc := tcinit(t)
			defer tc.cleanup()

			socketPath, srvStop := startHTTPServer(t, "/get/zip-1.15.5.tgz", tc.handler)
			defer srvStop()

			ut := &httpunix.Transport{}
			ut.RegisterLocation("notpecl", socketPath)
			httpT := &http.Transport{}
			httpT.RegisterProtocol(httpunix.Scheme, ut)

			tc.backend = tc.backend.WithHTTPTransport(httpT)
			tc.backend = tc.backend.WithBaseURI("http+unix://notpecl")

			ctx := context.TODO()
			_, err := tc.backend.Download(ctx, "zip", "1.15.5")
			if tc.expectedErr != nil {
				if err == nil || tc.expectedErr.Error() != err.Error() {
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
	backend       backends.PeclBackend
	configureArgs []string
	expectedErr   error
	cleanup       func()
}

func initInstallRedis511TC(
	t *testing.T,
	httpT *http.Transport,
	baseURI string,
) peclInstallTC {
	downloadDir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatal(err)
	}

	installDir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatal(err)
	}

	cleanup := func() {
		if err := os.RemoveAll(downloadDir); err != nil {
			t.Fatal(err)
		}
		if err := os.RemoveAll(installDir); err != nil {
			t.Fatal(err)
		}
	}

	np := backends.NewNotPeclBackend()
	b, err := backends.NewPeclBackend(np, downloadDir, downloadDir)
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
		"successfully install redis-5.1.1": initInstallRedis511TC,
	}

	for tcname := range testcases {
		tcinit := testcases[tcname]

		t.Run(tcname, func(t *testing.T) {
			t.Parallel()

			socketPath, srvStop := startHTTPServer(
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
			opts := backends.InstallOpts{
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

func serveExtTgzHandler(t *testing.T, fullpath string) http.Handler {
	buf, err := ioutil.ReadFile(fullpath)
	if err != nil {
		t.Fatal(err)
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Length", strconv.Itoa(len(buf)))
		w.WriteHeader(200)
		w.Write(buf)
	})
}
