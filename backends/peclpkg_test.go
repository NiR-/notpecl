package backends_test

import (
	"testing"

	"github.com/NiR-/notpecl/backends"
	"github.com/go-test/deep"
)

type loadTC struct {
	file        string
	expected    backends.Package
	expectedErr error
}

func TestLoadPackageXMLFromFile(t *testing.T) {
	testcases := map[string]loadTC{
		"successfully load package.xml for redis ext": {
			file: "testdata/package-redis.xml",
			expected: backends.Package{
				Name:        "redis",
				Summary:     "PHP extension for interfacing with Redis",
				Description: "This extension provides an API for communicating with Redis servers.",
				PublishDate: "2019-11-11",
				PublishTime: "07:36:41",
				Version: backends.Version{
					Release: "5.1.1",
					API:     "5.1.0",
				},
				Stability: backends.PackageStability{
					Release: backends.Stable,
					API:     backends.Stable,
				},
				License: backends.License{
					Name: "PHP",
					URI:  "http://www.php.net/license",
				},
				Dependencies: backends.Dependencies{
					Required: backends.RequiredDependencies{
						PHP: backends.PHPConstraint{
							Min: "7.0.0",
							Max: "7.9.99",
						},
					},
				},
				ExtSrcRelease: backends.ExtSrcRelease{
					ConfigureOptions: []backends.ConfigureOption{
						{
							Name:    "enable-redis-igbinary",
							Default: "no",
							Prompt:  "enable igbinary serializer support?",
						},
						{
							Name:    "enable-redis-lzf",
							Default: "no",
							Prompt:  "enable lzf compression support?",
						},
						{
							Name:    "enable-redis-zstd",
							Default: "no",
							Prompt:  "enable zstd compression support?",
						},
					},
				},
				Changelog: backends.Changelog{
					Releases: []backends.Release{
						{
							Stability: backends.PackageStability{
								Release: backends.Stable,
								API:     backends.Stable,
							},
							Version: backends.Version{
								Release: "5.1.1",
								API:     "5.1.0",
							},
							Date: "2019-11-11",
							Notes: `
phpredis 5.1.1

This release contains only bugfix for unix-socket connection.

* Fix fail to connect to redis through unix socket [2bae8010, 9f4ededa] (Pavlo Yatsukhnenko, Michael Grunder)
* Documentation improvements (@fitztrev)
   `,
						},
					},
				},
			},
		},
	}

	for _, tc := range testcases {
		out, err := backends.LoadPackageXMLFromFile(tc.file)
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

		// Match only the 1st release in the changelog as testing more
		// releases wouldn't bring anything more but would make this file
		// quite big.
		out.Changelog.Releases = out.Changelog.Releases[:1]

		if diff := deep.Equal(out, tc.expected); diff != nil {
			t.Fatal(diff)
		}
	}
}
