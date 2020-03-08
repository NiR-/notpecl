package peclapi_test

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/NiR-/notpecl/peclapi"
	"github.com/go-test/deep"
)

func newTestRoundTripper(
	t *testing.T,
	expectedURL string,
	statusCode int,
	body string,
) testRoundTripper {
	return func(req *http.Request) *http.Response {
		if req.URL.String() != expectedURL {
			t.Fatalf("Expected URL: %s - Got: %s", expectedURL, req.URL)
		}

		return &http.Response{
			StatusCode: statusCode,
			Body:       ioutil.NopCloser(bytes.NewBufferString(body)),
		}
	}
}

type testRoundTripper func(*http.Request) *http.Response

func (fn testRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req), nil
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

func loadTestdata(t *testing.T, filepath string) string {
	return string(loadRawTestdata(t, filepath))
}

type listPackagesTC struct {
	httpClient  *http.Client
	expected    []string
	expectedErr error
}

func initListAvailablePackagesTC(t *testing.T) listPackagesTC {
	expectedURL := "https://pecl.php.net/rest/p/packages.xml"
	body := loadTestdata(t, "testdata/list-packages.xml")
	roundTripper := newTestRoundTripper(t, expectedURL, 200, body)

	return listPackagesTC{
		httpClient: newTestClient(roundTripper),
		expected: []string{
			"amqp",
			"AOP",
			"ev",
		},
	}
}

func initFailToListWhenEndpointReturnsNon200TC(t *testing.T) listPackagesTC {
	expectedURL := "https://pecl.php.net/rest/p/packages.xml"
	roundTripper := newTestRoundTripper(t, expectedURL, 500, "")

	return listPackagesTC{
		httpClient:  newTestClient(roundTripper),
		expectedErr: fmt.Errorf("could not list packages: expected status code 200, got 500"),
	}
}

func initFailToListWhenEndpointReturnsBrokenXMLTC(t *testing.T) listPackagesTC {
	expectedURL := "https://pecl.php.net/rest/p/packages.xml"
	body := loadTestdata(t, "testdata/list-packages-broken.xml")
	roundTripper := newTestRoundTripper(t, expectedURL, 200, body)

	return listPackagesTC{
		httpClient:  newTestClient(roundTripper),
		expectedErr: fmt.Errorf("could not list packages: XML syntax error on line 9: unexpected EOF"),
	}
}

func TestListPackages(t *testing.T) {
	testcases := map[string]func(*testing.T) listPackagesTC{
		"successfully list available packages":           initListAvailablePackagesTC,
		"fail when endpoint reutrns non-200 status code": initFailToListWhenEndpointReturnsNon200TC,
		"fail when endopoint reutrns broken XML":         initFailToListWhenEndpointReturnsBrokenXMLTC,
	}

	for tcname := range testcases {
		tcinit := testcases[tcname]

		t.Run(tcname, func(t *testing.T) {
			t.Parallel()

			tc := tcinit(t)
			client := peclapi.NewClient(peclapi.WithHttpClient(tc.httpClient))

			list, err := client.ListPackages()
			if tc.expectedErr != nil {
				if err == nil || tc.expectedErr.Error() != err.Error() {
					t.Fatalf("Expected error: %v\nGot: %v", tc.expectedErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if diff := deep.Equal(list, tc.expected); diff != nil {
				t.Fatal(diff)
			}
		})
	}
}

func initListPackagesInDatabaseCategoryTC(t *testing.T) listPackagesTC {
	expectedURL := "https://pecl.php.net/rest/c/Database/packages.xml"
	body := loadTestdata(t, "testdata/category-database.xml")
	roundTripper := newTestRoundTripper(t, expectedURL, 200, body)

	return listPackagesTC{
		httpClient: newTestClient(roundTripper),
		expected:   []string{"sqlite3", "mongo"},
	}
}

func initFailToListDatabaseCategoryWhenEndpointReturnsBadStatusCodeTC(t *testing.T) listPackagesTC {
	expectedURL := "https://pecl.php.net/rest/c/Database/packages.xml"
	roundTripper := newTestRoundTripper(t, expectedURL, 500, "")

	return listPackagesTC{
		httpClient:  newTestClient(roundTripper),
		expectedErr: fmt.Errorf("could not list packages in Database category: expected status code 200, got 500"),
	}
}

func initFailToListDatabaseCategoryWhenEndpointReturns404TC(t *testing.T) listPackagesTC {
	expectedURL := "https://pecl.php.net/rest/c/Database/packages.xml"
	roundTripper := newTestRoundTripper(t, expectedURL, 404, "")

	return listPackagesTC{
		httpClient:  newTestClient(roundTripper),
		expectedErr: fmt.Errorf("could not list packages in Database category: category not found"),
	}
}

func initFailToListDatabaseCategoryWhenEndpointReturnsBrokenXMLTC(t *testing.T) listPackagesTC {
	expectedURL := "https://pecl.php.net/rest/c/Database/packages.xml"
	body := loadTestdata(t, "testdata/category-database-broken.xml")
	roundTripper := newTestRoundTripper(t, expectedURL, 200, body)

	return listPackagesTC{
		httpClient:  newTestClient(roundTripper),
		expectedErr: fmt.Errorf("could not list packages in Database category: XML syntax error on line 7: unexpected EOF"),
	}
}

func TestListPackagesInCategory(t *testing.T) {
	testcases := map[string]func(*testing.T) listPackagesTC{
		"successfully list available packages in database category": initListPackagesInDatabaseCategoryTC,
		"fail when endpoint reutrns a 404":                          initFailToListDatabaseCategoryWhenEndpointReturns404TC,
		"fail when endpoint reutrns a bad status code":              initFailToListDatabaseCategoryWhenEndpointReturnsBadStatusCodeTC,
		"fail when endopoint reutrns broken XML":                    initFailToListDatabaseCategoryWhenEndpointReturnsBrokenXMLTC,
	}

	for tcname := range testcases {
		tcinit := testcases[tcname]

		t.Run(tcname, func(t *testing.T) {
			t.Parallel()

			tc := tcinit(t)
			client := peclapi.NewClient(peclapi.WithHttpClient(tc.httpClient))

			list, err := client.ListPackagesInCategory("Database")
			if tc.expectedErr != nil {
				if err == nil || tc.expectedErr.Error() != err.Error() {
					t.Fatalf("Expected error: %v\nGot: %v", tc.expectedErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if diff := deep.Equal(list, tc.expected); diff != nil {
				t.Fatal(diff)
			}
		})
	}
}

type describePackageTC struct {
	httpClient  *http.Client
	expected    peclapi.Package
	expectedErr error
}

func initSuccessfullyDescribePackageTC(t *testing.T) describePackageTC {
	expectedURL := "https://pecl.php.net/rest/p/redis/info.xml"
	body := loadTestdata(t, "testdata/describe-redis.xml")
	roundTripper := newTestRoundTripper(t, expectedURL, 200, body)

	return describePackageTC{
		httpClient: newTestClient(roundTripper),
		expected: peclapi.Package{
			Name:        "redis",
			Category:    "Database",
			License:     "PHP",
			Summary:     "PHP extension for interfacing with Redis",
			Description: "This extension provides an API for communicating with Redis servers.",
		},
	}
}

func initFailToDescribePackageWhenEndpointReturns404TC(t *testing.T) describePackageTC {
	expectedURL := "https://pecl.php.net/rest/p/redis/info.xml"
	roundTripper := newTestRoundTripper(t, expectedURL, 404, "")

	return describePackageTC{
		httpClient:  newTestClient(roundTripper),
		expectedErr: fmt.Errorf("could not describe package redis: package not found"),
	}
}

func initFailToDescribePackageWhenEndpointFailsTC(t *testing.T) describePackageTC {
	expectedURL := "https://pecl.php.net/rest/p/redis/info.xml"
	roundTripper := newTestRoundTripper(t, expectedURL, 500, "")

	return describePackageTC{
		httpClient:  newTestClient(roundTripper),
		expectedErr: fmt.Errorf("could not describe package redis: expected status code 200, got 500"),
	}
}

func initFailToDescribePackageWhenEndpointReturnsBadXMLTC(t *testing.T) describePackageTC {
	expectedURL := "https://pecl.php.net/rest/p/redis/info.xml"
	body := loadTestdata(t, "testdata/describe-redis-broken.xml")
	roundTripper := newTestRoundTripper(t, expectedURL, 200, body)

	return describePackageTC{
		httpClient:  newTestClient(roundTripper),
		expectedErr: fmt.Errorf("could not describe package redis: XML syntax error on line 8: unexpected EOF"),
	}
}

func TestDescribePackage(t *testing.T) {
	testcases := map[string]func(*testing.T) describePackageTC{
		"successfully describe package":                          initSuccessfullyDescribePackageTC,
		"fail to describe package when endpoint returns 404":     initFailToDescribePackageWhenEndpointReturns404TC,
		"fail to describe package when endpoint fails":           initFailToDescribePackageWhenEndpointFailsTC,
		"fail to describe package when endpoint returns bad xml": initFailToDescribePackageWhenEndpointReturnsBadXMLTC,
	}

	for tcname := range testcases {
		tcinit := testcases[tcname]

		t.Run(tcname, func(t *testing.T) {
			t.Parallel()

			tc := tcinit(t)
			client := peclapi.NewClient(peclapi.WithHttpClient(tc.httpClient))

			pkg, err := client.DescribePackage("redis")
			if tc.expectedErr != nil {
				if err.Error() != tc.expectedErr.Error() {
					t.Fatalf("Expected error: %v\nGot: %v", tc.expectedErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if diff := deep.Equal(pkg, tc.expected); diff != nil {
				t.Fatal(diff)
			}
		})
	}
}

type listReleasesTC struct {
	httpClient  *http.Client
	expected    peclapi.PackageReleases
	expectedErr error
}

func initListAvailableReleasesTC(t *testing.T) listReleasesTC {
	expectedURL := "https://pecl.php.net/rest/r/redis/allreleases.xml"
	body := loadTestdata(t, "testdata/list-releases.xml")
	roundTripper := newTestRoundTripper(t, expectedURL, 200, body)

	return listReleasesTC{
		httpClient: newTestClient(roundTripper),
		expected: peclapi.PackageReleases(map[string]peclapi.Stability{
			"5.2.0":    peclapi.Stable,
			"5.2.0RC2": peclapi.Alpha,
			"5.2.0RC1": peclapi.Alpha,
			"5.1.1":    peclapi.Stable,
			"5.1.0":    peclapi.Stable,
			"5.1.0RC2": peclapi.Beta,
			"5.1.0RC1": peclapi.Alpha,
		}),
	}
}

func initFailToListReleasesWhenPackageIsNotFoundTC(t *testing.T) listReleasesTC {
	expectedURL := "https://pecl.php.net/rest/r/redis/allreleases.xml"
	roundTripper := newTestRoundTripper(t, expectedURL, 404, "")

	return listReleasesTC{
		httpClient:  newTestClient(roundTripper),
		expectedErr: fmt.Errorf("could not list releases for redis: package not found"),
	}
}

func initFailToListReleasesWhenEndpointReturnsNon200TC(t *testing.T) listReleasesTC {
	expectedURL := "https://pecl.php.net/rest/r/redis/allreleases.xml"
	roundTripper := newTestRoundTripper(t, expectedURL, 500, "")

	return listReleasesTC{
		httpClient:  newTestClient(roundTripper),
		expectedErr: fmt.Errorf("could not list releases for redis: expected status code 200, got 500"),
	}
}

func initFailToListReleasesWhenEndpointReturnsBrokenXMLTC(t *testing.T) listReleasesTC {
	expectedURL := "https://pecl.php.net/rest/r/redis/allreleases.xml"
	body := loadTestdata(t, "testdata/list-releases-broken.xml")
	roundTripper := newTestRoundTripper(t, expectedURL, 200, body)

	return listReleasesTC{
		httpClient:  newTestClient(roundTripper),
		expectedErr: fmt.Errorf("could not list releases for redis: XML syntax error on line 13: unexpected EOF"),
	}
}

func TestListReleases(t *testing.T) {
	testcases := map[string]func(*testing.T) listReleasesTC{
		"successfully list available releases":           initListAvailableReleasesTC,
		"fail when package is not found":                 initFailToListReleasesWhenPackageIsNotFoundTC,
		"fail when endpoint reutrns non-200 status code": initFailToListReleasesWhenEndpointReturnsNon200TC,
		"fail when endopoint reutrns broken XML":         initFailToListReleasesWhenEndpointReturnsBrokenXMLTC,
	}

	for tcname := range testcases {
		tcinit := testcases[tcname]

		t.Run(tcname, func(t *testing.T) {
			t.Parallel()

			tc := tcinit(t)
			client := peclapi.NewClient(peclapi.WithHttpClient(tc.httpClient))

			releases, err := client.ListReleases("redis")
			if tc.expectedErr != nil {
				if err == nil || tc.expectedErr.Error() != err.Error() {
					t.Fatalf("Expected error: %v\nGot: %v", tc.expectedErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if diff := deep.Equal(releases, tc.expected); diff != nil {
				t.Fatal(diff)
			}
		})
	}
}

type describeReleaseTC struct {
	httpClient  *http.Client
	expected    peclapi.Release
	expectedErr error
}

func initDescribeReleaseTC(t *testing.T) describeReleaseTC {
	expectedURL := "https://pecl.php.net/rest/r/redis/5.2.0.xml"
	body := loadTestdata(t, "testdata/redis-5.2.0.xml")
	roundTripper := newTestRoundTripper(t, expectedURL, 200, body)

	return describeReleaseTC{
		httpClient: newTestClient(roundTripper),
		expected: peclapi.Release{
			Package:     "redis",
			Version:     "5.2.0",
			Stability:   "stable",
			License:     "PHP",
			Maintainer:  "mgrunder",
			Summary:     "PHP extension for interfacing with Redis",
			Description: "This extension provides an API for communicating with Redis servers.",
			PartialURI:  "https://pecl.php.net/get/redis-5.2.0",
			ReleaseDate: "2020-03-02 06:16:57",
			ReleaseNotes: `phpredis 5.2.0

- There were no changes between 5.2.0RC2 and 5.2.0.

phpredis 5.2.0RC2

* Include RedisSentinelTest.php in package.xml! [eddbfc8f] (Michael Grunder)
* Fix -Wmaybe-uninitialized warning [740b8c87] (Remi Collet)
* Fix improper destructor when zipping values and scores [371ae7ae]
  (Michael Grunder)
* Use php_rand instead of php_mt_rand for liveness challenge string
  [9ef2ed89] (Michael Grunder)

phpredis 5.2.0RC1

This release contains initial support for Redis Sentinel as well as many
smaller bug fixes and improvements.  It is especially of interest if you
use persistent connections, as we've added logic to make sure they are in
a good state when retreving them from the pool.

IMPORTANT: Sentinel support is considered experimental and the API
           will likely change based on user feedback.

* Sponsors
  ~ Audiomack.com - https://audiomack.com
  ~ Till Kruss - https://github.com/tillkruss

---

* Initial support for RedisSentinel [90cb69f3, c94e28f1, 46da22b0, 5a609fa4,
  383779ed] (Pavlo Yatsukhnenko)

* Houskeeping (spelling, doc changes, etc) [23f9de30, d07a8df6, 2d39b48d,
  0ef488fc, 2c35e435, f52bd8a8, 2ddc5f21, 1ff7dfb7, db446138] (Tyson Andre,
  Pavlo Yatsukhnenko, Michael Grunder, Tyson Andre)

* Fix for ASK redirections [ba73fbee] (Michael Grunder)
* Create specific 'test skipped' exception [c3d83d44] (Michael Grunder)
* Fixed memory leaks in RedisCluster [a107c9fc] (Michael Grunder)
* Fixes for session lifetime values that underflow or overflow  [7a79ad9c,
  3c48a332] (Michael Grunder)
* Enables slot caching for Redis Cluster [23b1a9d8] (Michael Booth)

* Support TYPE argument for SCAN [8eb39a26, b1724b84, 53fb36c9, 544e641b]
  (Pavlo Yatsukhnenko)

* Added challenge/response mechanism for persistent connections [a5f95925,
  25cdaee6, 7b6072e0, 99ebd0cc, 3243f426] (Pavlo Yatsukhnenko, Michael Grunder)`,
		},
	}
}
func initFailToDescribeReleaseWhenEndpointReturns404TC(t *testing.T) describeReleaseTC {
	expectedURL := "https://pecl.php.net/rest/r/redis/5.2.0.xml"
	roundTripper := newTestRoundTripper(t, expectedURL, 404, "")

	return describeReleaseTC{
		httpClient:  newTestClient(roundTripper),
		expectedErr: fmt.Errorf("could not describe redis release 5.2.0: release not found"),
	}
}

func initFailToDescribeReleaseWhenEndpointFailsTC(t *testing.T) describeReleaseTC {
	expectedURL := "https://pecl.php.net/rest/r/redis/5.2.0.xml"
	roundTripper := newTestRoundTripper(t, expectedURL, 500, "")

	return describeReleaseTC{
		httpClient:  newTestClient(roundTripper),
		expectedErr: fmt.Errorf("could not describe redis releases 5.2.0: expected status code 200, got 500"),
	}
}

func initFailToDescribeReleaseWhenEndpointReturnsBadXMLTC(t *testing.T) describeReleaseTC {
	expectedURL := "https://pecl.php.net/rest/r/redis/5.2.0.xml"
	body := loadTestdata(t, "testdata/redis-5.2.0-broken.xml")
	roundTripper := newTestRoundTripper(t, expectedURL, 200, body)

	return describeReleaseTC{
		httpClient:  newTestClient(roundTripper),
		expectedErr: fmt.Errorf("could not describe redis release 5.2.0: XML syntax error on line 14: unexpected EOF"),
	}
}

func TestDescribeRelease(t *testing.T) {
	testcases := map[string]func(*testing.T) describeReleaseTC{
		"successfully describe redis v5.2.0":                     initDescribeReleaseTC,
		"fail to describe release when endpoint returns 404":     initFailToDescribeReleaseWhenEndpointReturns404TC,
		"fail to describe release when endpoint fails":           initFailToDescribeReleaseWhenEndpointFailsTC,
		"fail to describe release when endpoint returns bad XML": initFailToDescribeReleaseWhenEndpointReturnsBadXMLTC,
	}

	for tcname := range testcases {
		tcinit := testcases[tcname]

		t.Run(tcname, func(t *testing.T) {
			t.Parallel()

			tc := tcinit(t)
			client := peclapi.NewClient(peclapi.WithHttpClient(tc.httpClient))

			release, err := client.DescribeRelease("redis", "5.2.0")
			if tc.expectedErr != nil {
				if err == nil || tc.expectedErr.Error() != err.Error() {
					t.Fatalf("Expected error: %v\nGot: %v", tc.expectedErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if diff := deep.Equal(release, tc.expected); diff != nil {
				t.Fatal(diff)
			}
		})
	}
}

type downloadReleaseTC struct {
	release     peclapi.Release
	httpClient  *http.Client
	expected    io.Reader
	expectedErr error
}

func initSuccessfullyDownloadReleaseTC(t *testing.T) downloadReleaseTC {
	expectedURL := "https://pecl.php.net/get/redis-5.1.1.tgz"
	body := loadRawTestdata(t, "testdata/redis-5.1.1.tgz")

	roundTripper := func(req *http.Request) *http.Response {
		if req.URL.String() != expectedURL {
			t.Fatalf("Expected URL: %s - Got: %s", expectedURL, req.URL)
		}

		return &http.Response{
			StatusCode:    200,
			Body:          ioutil.NopCloser(bytes.NewBuffer(body)),
			ContentLength: int64(len(body)),
		}
	}

	return downloadReleaseTC{
		release: peclapi.Release{
			Package:    "redis",
			Version:    "5.1.1",
			PartialURI: "https://pecl.php.net/get/redis-5.1.1",
		},
		httpClient: newTestClient(roundTripper),
		expected:   bufio.NewReader(bytes.NewBuffer(body)),
	}
}

func initFailToDownloadReleaseWhenStatusCodeIsNot200TC(t *testing.T) downloadReleaseTC {
	expectedURL := "https://pecl.php.net/get/redis-5.1.1.tgz"

	roundTripper := func(req *http.Request) *http.Response {
		if req.URL.String() != expectedURL {
			t.Fatalf("Expected URL: %s - Got: %s", expectedURL, req.URL)
		}

		return &http.Response{
			StatusCode: 500,
			Body:       ioutil.NopCloser(bytes.NewBufferString("")),
		}
	}

	return downloadReleaseTC{
		release: peclapi.Release{
			Package:    "redis",
			Version:    "5.1.1",
			PartialURI: "https://pecl.php.net/get/redis-5.1.1",
		},
		httpClient:  newTestClient(roundTripper),
		expectedErr: fmt.Errorf("could not download redis v5.1.1: expected status code 200, got 500"),
	}
}

func initFailToDownloadReleaseWhenDownloadedFileHasWrongMimeType(t *testing.T) downloadReleaseTC {
	expectedURL := "https://pecl.php.net/get/redis-5.1.1.tgz"
	// redis-5.2.0.xml is used here because it's more than 64-bytes long and
	// this test requires any file that's not a gzip file.
	body := loadRawTestdata(t, "testdata/redis-5.2.0.xml")

	roundTripper := func(req *http.Request) *http.Response {
		if req.URL.String() != expectedURL {
			t.Fatalf("Expected URL: %s - Got: %s", expectedURL, req.URL)
		}

		return &http.Response{
			StatusCode:    200,
			Body:          ioutil.NopCloser(bytes.NewBuffer(body)),
			ContentLength: int64(len(body)),
		}
	}

	return downloadReleaseTC{
		release: peclapi.Release{
			Package:    "redis",
			Version:    "5.1.1",
			PartialURI: "https://pecl.php.net/get/redis-5.1.1",
		},
		httpClient:  newTestClient(roundTripper),
		expectedErr: fmt.Errorf("the file downloaded at https://pecl.php.net/get/redis-5.1.1.tgz isn't a gzip file"),
	}
}

func TestDownloadRelease(t *testing.T) {
	testcases := map[string]func(*testing.T) downloadReleaseTC{
		"successfully download release":                  initSuccessfullyDownloadReleaseTC,
		"fails when status code is not 200":              initFailToDownloadReleaseWhenStatusCodeIsNot200TC,
		"fails when downloaded file has wrong mime type": initFailToDownloadReleaseWhenDownloadedFileHasWrongMimeType,
	}

	for tcname := range testcases {
		tcinit := testcases[tcname]

		t.Run(tcname, func(t *testing.T) {
			t.Parallel()

			tc := tcinit(t)
			client := peclapi.NewClient(peclapi.WithHttpClient(tc.httpClient))

			tarr, err := client.DownloadRelease(tc.release)
			if tc.expectedErr != nil {
				if err == nil || err.Error() != tc.expectedErr.Error() {
					t.Fatalf("Expected error: %v\nGot: %v", tc.expectedErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if diff := deep.Equal(tarr, tc.expected); diff != nil {
				t.Fatal(diff)
			}
		})
	}
}
