package composerlock

import (
	"os"
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name    string
		file    string
		want    Lock
		wantErr bool
	}{
		{
			name: "realistic lock file with prod and dev packages",
			file: "testdata/realistic.json",
			want: Lock{
				Packages: []Package{
					{
						Name:      "symfony/console",
						Version:   "v6.4.3",
						Reference: "abcdef1234567890",
						SourceURL: "https://github.com/symfony/console",
					},
				},
				PackagesDev: []Package{
					{
						Name:      "phpunit/phpunit",
						Version:   "10.5.9",
						Reference: "1234567890abcdef",
						SourceURL: "https://github.com/sebastianbergmann/phpunit",
					},
				},
			},
		},
		{
			name: "packages missing source",
			file: "testdata/missing_source.json",
			want: Lock{
				Packages: []Package{
					{
						Name:    "acme/metapackage",
						Version: "1.0.0",
					},
					{
						Name:    "acme/path-package",
						Version: "dev-main",
					},
				},
			},
		},
		{
			name: "dev-branch package with a reference",
			file: "testdata/dev_branch.json",
			want: Lock{
				Packages: []Package{
					{
						Name:      "acme/dev-dependency",
						Version:   "dev-main",
						Reference: "fedcba0987654321",
						SourceURL: "https://github.com/acme/dev-dependency",
					},
				},
			},
		},
		{
			name: "empty lock file with neither packages array populated",
			file: "testdata/empty.json",
			want: Lock{},
		},
		{
			name:    "malformed JSON",
			file:    "testdata/malformed.json",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := os.ReadFile(tt.file)
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}

			got, err := Parse(data)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("Parse() error = nil, want an error")
				}
				return
			}
			if err != nil {
				t.Fatalf("Parse() unexpected error: %v", err)
			}

			assertPackagesEqual(t, "Packages", got.Packages, tt.want.Packages)
			assertPackagesEqual(t, "PackagesDev", got.PackagesDev, tt.want.PackagesDev)
		})
	}
}

func assertPackagesEqual(t *testing.T, field string, got, want []Package) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("%s length = %d, want %d (got %#v)", field, len(got), len(want), got)
	}

	for i := range want {
		if got[i] != want[i] {
			t.Errorf("%s[%d] = %#v, want %#v", field, i, got[i], want[i])
		}
	}
}
