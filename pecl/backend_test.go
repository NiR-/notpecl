package pecl_test

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"testing"

	"github.com/NiR-/notpecl/cmdexec"
	"github.com/NiR-/notpecl/pecl"
	"github.com/NiR-/notpecl/peclapi"
	"github.com/twpayne/go-vfs/vfst"
)

var phpconfigPath string

func init() {
	if cmdexec.IsMockbin() {
		cmdexec.Mockbin()
	}

	phpconfigPath = os.Getenv("PHPCONFIG_PATH")
	if phpconfigPath == "" {
		phpconfigPath = "/usr/bin/php-config"
	}
}

func newTestRoundTripper(
	t *testing.T,
	resps map[string][]byte,
) testRoundTripper {
	return func(req *http.Request) *http.Response {
		for url, body := range resps {
			if req.URL.String() != url {
				continue
			}

			return &http.Response{
				StatusCode:    200,
				Body:          ioutil.NopCloser(bytes.NewBuffer(body)),
				ContentLength: int64(len(body)),
			}
		}

		t.Fatalf("No matching resps found for %s", req.URL)
		return nil
	}
}

type testRoundTripper func(*http.Request) *http.Response

func (fn testRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req), nil
}

func newFailingTestRoundTripper(t *testing.T, err error) testFailingRoundTripper {
	return testFailingRoundTripper{err}
}

type testFailingRoundTripper struct {
	err error
}

func (err testFailingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return nil, err.err
}

func newTestClient(fn testRoundTripper) *http.Client {
	return &http.Client{
		Transport: fn,
	}
}

func loadRawTestdata(t *testing.T, filepath string) []byte {
	raw, err := ioutil.ReadFile(filepath)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

type resolveConstraintTC struct {
	httpClient       *http.Client
	extension        string
	constraint       string
	minimumStability peclapi.Stability
	expected         string
	expectedErr      error
}

func initSuccessfullyResolveLastStableVersionTC(t *testing.T) resolveConstraintTC {
	body := loadRawTestdata(t, "testdata/redis-releases.xml")
	roundTripper := newTestRoundTripper(t, map[string][]byte{
		"https://pecl.php.net/rest/r/redis/allreleases.xml": body,
	})

	return resolveConstraintTC{
		httpClient:       newTestClient(roundTripper),
		extension:        "redis",
		constraint:       "*",
		minimumStability: peclapi.Stable,
		expected:         "5.2.0",
	}
}

func initFailToResolveWhenClientFailsTC(t *testing.T) resolveConstraintTC {
	roundTripper := newFailingTestRoundTripper(t, fmt.Errorf("some error"))

	return resolveConstraintTC{
		httpClient:       &http.Client{Transport: roundTripper},
		extension:        "redis",
		constraint:       "*",
		minimumStability: peclapi.Stable,
		expectedErr:      fmt.Errorf("could not resolve constraint for redis: Get \"https://pecl.php.net/rest/r/redis/allreleases.xml\": some error"),
	}
}

func TestResolveConstraint(t *testing.T) {
	testcases := map[string]func(*testing.T) resolveConstraintTC{
		"successfully resolve last stable version":         initSuccessfullyResolveLastStableVersionTC,
		"fail to resolve constraint when API client fails": initFailToResolveWhenClientFailsTC,
	}

	for tcname := range testcases {
		tcinit := testcases[tcname]

		t.Run(tcname, func(t *testing.T) {
			t.Parallel()

			tc := tcinit(t)
			client := peclapi.NewClient(peclapi.WithHttpClient(tc.httpClient))
			backend := pecl.New(pecl.WithClient(client))

			resolved, err := backend.ResolveConstraint(tc.extension, tc.constraint, tc.minimumStability)
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
				t.Fatalf("Expected resolved version: %s - Got: %s", tc.expected, resolved)
			}
		})
	}
}

type peclDownloadTC struct {
	httpClient   *http.Client
	downloadOpts pecl.DownloadOpts
	// expected is the path where the extensions should be downloaded
	expected    string
	fsTests     []interface{}
	expectedErr error
}

func initSuccessfullyDownloadZipV1155TC(t *testing.T) peclDownloadTC {
	releases := loadRawTestdata(t, "testdata/zip-release-1.15.5.xml")
	tgz := loadRawTestdata(t, "testdata/zip-1.15.5.tgz")
	roundTripper := newTestRoundTripper(t, map[string][]byte{
		"https://pecl.php.net/rest/r/zip/1.15.5.xml": releases,
		"https://pecl.php.net/get/zip-1.15.5.tgz":    tgz,
	})

	return peclDownloadTC{
		httpClient: newTestClient(roundTripper),
		downloadOpts: pecl.DownloadOpts{
			Extension:   "zip",
			Version:     "1.15.5",
			DownloadDir: "/tmp",
		},
		expected: "/tmp/zip-1.15.5",
		fsTests: []interface{}{
			vfst.TestPath("/tmp/zip-1.15.5", vfst.TestIsDir),
		},
	}
}

func TestDownload(t *testing.T) {
	testcases := map[string]func(*testing.T) peclDownloadTC{
		"successfully download zip v1.15.5": initSuccessfullyDownloadZipV1155TC,
	}

	for tcname := range testcases {
		tcinit := testcases[tcname]

		t.Run(tcname, func(t *testing.T) {
			t.Parallel()

			tc := tcinit(t)
			client := peclapi.NewClient(peclapi.WithHttpClient(tc.httpClient))

			fs, cleanup, err := vfst.NewTestFS(map[string]interface{}{
				tc.downloadOpts.DownloadDir: &vfst.Dir{
					Perm: 0750,
				},
			})
			if err != nil {
				t.Fatalf("Unexpected error: %+v", err)
			}
			defer cleanup()

			backend := pecl.New(
				pecl.WithClient(client),
				pecl.WithFS(fs))

			outpath, err := backend.Download(tc.downloadOpts)
			if tc.expectedErr != nil {
				if err == nil || err.Error() != tc.expectedErr.Error() {
					t.Fatalf("Expected error: %v\nGot: %v", tc.expectedErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if outpath != tc.expected {
				t.Fatalf("Expected outpath path: %s - Got: %s", tc.expected, outpath)
			}

			vfst.RunTests(t, fs, "downloaded file", tc.fsTests...)
		})
	}
}

type installTC struct {
	httpClient  *http.Client
	cmdExec     cmdexec.CmdExecutor
	recorder    *cmdexec.Recorder
	cmdTester   cmdexec.Tester
	opts        pecl.InstallOpts
	expectedErr error
}

func initSuccessfullyInstallZipTC(t *testing.T) installTC {
	releases := loadRawTestdata(t, "testdata/zip-release-1.15.5.xml")
	tgz := loadRawTestdata(t, "testdata/zip-1.15.5.tgz")
	roundTripper := newTestRoundTripper(t, map[string][]byte{
		"https://pecl.php.net/rest/r/zip/1.15.5.xml": releases,
		"https://pecl.php.net/get/zip-1.15.5.tgz":    tgz,
	})

	executor, recorder := cmdexec.NewTestExecutor()
	executor = executor.With(
		cmdexec.FakeOn([]string{"php", "-r", "echo json_encode(PHP_VERSION);"},
			cmdexec.FakeStdout("\"7.4.3\"")),
	)

	cmdTester := cmdexec.BuildTesters(
		cmdexec.ExpectCommandArgs([]string{"phpize"}),
		cmdexec.ExpectCommandArgs([]string{
			"./configure",
			"--with-php-config=" + phpconfigPath}),
		cmdexec.ExpectCommandArgs([]string{"make"}),
		cmdexec.ExpectCommandArgs([]string{
			"make",
			"INSTALL_ROOT=/installdir",
			"install"}),
		cmdexec.ExpectCommandArgs([]string{"make", "clean"}))

	return installTC{
		httpClient: newTestClient(roundTripper),
		cmdExec:    executor,
		recorder:   recorder,
		cmdTester:  cmdTester,
		opts: pecl.InstallOpts{
			DownloadOpts: pecl.DownloadOpts{
				Extension:   "zip",
				Version:     "1.15.5",
				DownloadDir: "/tmp",
			},
			InstallDir: "/installdir",
			Cleanup:    true,
		},
	}
}

func initSuccessfullyInstallRedisWithArgsTC(t *testing.T) installTC {
	releases := loadRawTestdata(t, "testdata/redis-release-5.1.1.xml")
	tgz := loadRawTestdata(t, "testdata/redis-5.1.1.tgz")
	roundTripper := newTestRoundTripper(t, map[string][]byte{
		"https://pecl.php.net/rest/r/redis/5.1.1.xml": releases,
		"https://pecl.php.net/get/redis-5.1.1.tgz":    tgz,
	})

	executor, recorder := cmdexec.NewTestExecutor()
	executor = executor.With(
		cmdexec.FakeOn([]string{"php", "-r", "echo json_encode(PHP_VERSION);"},
			cmdexec.FakeStdout("\"7.4.3\"")),
	)
	cmdTester := cmdexec.BuildTesters(
		cmdexec.ExpectCommandArgs([]string{"phpize"}),
		cmdexec.ExpectCommandArgs([]string{
			"./configure",
			"--enable-redis-lzf",
			"--enable-redis-igbinary=no",
			"--enable-redis-zstd=no",
			"--with-php-config=" + phpconfigPath}),
		cmdexec.ExpectCommandArgs([]string{"make"}),
		cmdexec.ExpectCommandArgs([]string{
			"make",
			"INSTALL_ROOT=/installdir",
			"install"}),
		cmdexec.ExpectCommandArgs([]string{"make", "clean"}))

	return installTC{
		httpClient: newTestClient(roundTripper),
		cmdExec:    executor,
		recorder:   recorder,
		cmdTester:  cmdTester,
		opts: pecl.InstallOpts{
			DownloadOpts: pecl.DownloadOpts{
				Extension:   "redis",
				Version:     "5.1.1",
				DownloadDir: "/tmp",
			},
			ConfigureArgs: []string{"--enable-redis-lzf"},
			InstallDir:    "/installdir",
			Cleanup:       true,
		},
	}
}

func TestInstall(t *testing.T) {
	testcases := map[string]func(*testing.T) installTC{
		"successfully install zip v1.15.5":            initSuccessfullyInstallZipTC,
		"successfully install redis v5.1.1 with args": initSuccessfullyInstallRedisWithArgsTC,
	}

	for tcname := range testcases {
		tcinit := testcases[tcname]

		t.Run(tcname, func(t *testing.T) {
			t.Parallel()

			tc := tcinit(t)
			fs, cleanup, err := vfst.NewTestFS(map[string]interface{}{
				tc.opts.DownloadDir: &vfst.Dir{Perm: 0750},
				tc.opts.InstallDir:  &vfst.Dir{Perm: 0750},
			})
			if err != nil {
				t.Fatal(err)
			}
			defer cleanup()

			client := peclapi.NewClient(peclapi.WithHttpClient(tc.httpClient))

			backend := pecl.New(
				pecl.WithFS(fs),
				pecl.WithClient(client),
				pecl.WithCmdExec(tc.cmdExec))

			err = backend.Install(tc.opts)
			if tc.expectedErr != nil {
				if err == nil || err.Error() != tc.expectedErr.Error() {
					t.Fatalf("Expected error: %v\nGot: %v", tc.expectedErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			tc.cmdTester(t, tc.recorder)
		})
	}
}
