package lockfile

import (
	"strings"
	"testing"
)

func TestParseEcosystem(t *testing.T) {
	tests := []struct {
		name    string
		token   string
		want    Ecosystem
		wantErr string // non-empty substring expected in the error, "" means no error
	}{
		{name: "composer lowercase", token: "composer", want: Composer},
		{name: "npm lowercase", token: "npm", want: NPM},
		{name: "pnpm lowercase", token: "pnpm", want: Pnpm},
		{name: "yarn lowercase", token: "yarn", want: Yarn},
		{name: "case-insensitive Composer", token: "Composer", want: Composer},
		{name: "case-insensitive PNPM", token: "PNPM", want: Pnpm},
		{name: "case-insensitive YARN", token: "YARN", want: Yarn},
		{name: "typo is rejected", token: "pmpm", wantErr: "unknown ecosystem"},
		{name: "empty token is rejected", token: "", wantErr: "unknown ecosystem"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseEcosystem(tt.token)

			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("ParseEcosystem(%q) error = nil, want error containing %q", tt.token, tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("ParseEcosystem(%q) error = %q, want it to contain %q", tt.token, err.Error(), tt.wantErr)
				}
				// The error lists the accepted tokens so a confused operator can fix it.
				for _, want := range []string{"composer", "npm", "pnpm", "yarn"} {
					if !strings.Contains(err.Error(), want) {
						t.Errorf("ParseEcosystem(%q) error = %q, want it to list the accepted token %q", tt.token, err.Error(), want)
					}
				}
				return
			}

			if err != nil {
				t.Fatalf("ParseEcosystem(%q) unexpected error: %v", tt.token, err)
			}
			if got != tt.want {
				t.Fatalf("ParseEcosystem(%q) = %v, want %v", tt.token, got, tt.want)
			}
		})
	}
}

func TestEcosystemSet(t *testing.T) {
	if !EcosystemSet(0).IsEmpty() {
		t.Error("zero-value EcosystemSet should be empty")
	}

	set := EcosystemSet(0).With(Composer).With(Pnpm)

	if set.IsEmpty() {
		t.Error("set with members should not report empty")
	}
	if !set.Contains(Composer) {
		t.Error("set should contain Composer")
	}
	if !set.Contains(Pnpm) {
		t.Error("set should contain Pnpm")
	}
	if set.Contains(NPM) {
		t.Error("set should not contain NPM")
	}

	// Adding an already-present member is a no-op, keeping the set a stable
	// comparable value.
	if set.With(Composer) != set {
		t.Error("adding an existing member should leave the set unchanged")
	}
}
