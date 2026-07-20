package semver

import "testing"

func TestCompare(t *testing.T) {
	tests := []struct {
		name   string
		a, b   string
		want   int
		wantOk bool
	}{
		{
			name:   "equal patch versions",
			a:      "1.2.3",
			b:      "1.2.3",
			want:   0,
			wantOk: true,
		},
		{
			name:   "patch upgrade",
			a:      "1.0.0",
			b:      "1.0.1",
			want:   -1,
			wantOk: true,
		},
		{
			name:   "minor downgrade",
			a:      "1.1.0",
			b:      "1.0.0",
			want:   1,
			wantOk: true,
		},
		{
			name:   "major upgrade",
			a:      "1.9.9",
			b:      "2.0.0",
			want:   -1,
			wantOk: true,
		},
		{
			name:   "numeric components compare numerically, not lexically",
			a:      "1.9.0",
			b:      "1.10.0",
			want:   -1,
			wantOk: true,
		},
		{
			name:   "Composer style v prefix on both sides",
			a:      "v6.4.2",
			b:      "v6.4.3",
			want:   -1,
			wantOk: true,
		},
		{
			name:   "v prefix on one side only still compares",
			a:      "6.4.2",
			b:      "v6.4.3",
			want:   -1,
			wantOk: true,
		},
		{
			name:   "missing patch is treated as zero",
			a:      "5.4",
			b:      "5.4.0",
			want:   0,
			wantOk: true,
		},
		{
			name:   "missing minor and patch are treated as zero",
			a:      "3",
			b:      "3.0.0",
			want:   0,
			wantOk: true,
		},
		{
			name:   "two component version compares against three component one",
			a:      "5.4",
			b:      "5.4.1",
			want:   -1,
			wantOk: true,
		},
		{
			name:   "build metadata is ignored",
			a:      "1.0.0+build9",
			b:      "1.0.0",
			want:   0,
			wantOk: true,
		},
		{
			name:   "pre-release sorts before its release",
			a:      "1.0.0-rc.1",
			b:      "1.0.0",
			want:   -1,
			wantOk: true,
		},
		{
			name:   "pre-release identifiers compare in spec order",
			a:      "1.0.0-alpha",
			b:      "1.0.0-beta",
			want:   -1,
			wantOk: true,
		},
		{
			name:   "numeric pre-release identifiers compare numerically",
			a:      "1.0.0-rc.2",
			b:      "1.0.0-rc.10",
			want:   -1,
			wantOk: true,
		},
		{
			name:   "fewer pre-release identifiers sorts first",
			a:      "1.0.0-alpha",
			b:      "1.0.0-alpha.1",
			want:   -1,
			wantOk: true,
		},
		{
			name:   "Composer dev branch alias is not comparable",
			a:      "dev-main",
			b:      "dev-main",
			wantOk: false,
		},
		{
			name:   "Composer dev suffix is not comparable",
			a:      "1.0.x-dev",
			b:      "1.1.x-dev",
			wantOk: false,
		},
		{
			name:   "pnpm workspace protocol is not comparable",
			a:      "workspace:*",
			b:      "1.0.0",
			wantOk: false,
		},
		{
			name:   "git URL is not comparable",
			a:      "1.0.0",
			b:      "git+https://example.com/pkg.git#abc1234",
			wantOk: false,
		},
		{
			name:   "empty version is not comparable",
			a:      "",
			b:      "1.0.0",
			wantOk: false,
		},
		{
			name:   "both empty is not comparable",
			a:      "",
			b:      "",
			wantOk: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, gotOk := Compare(tt.a, tt.b)

			if gotOk != tt.wantOk {
				t.Fatalf("Compare(%q, %q) ok = %v, want %v", tt.a, tt.b, gotOk, tt.wantOk)
			}

			if got != tt.want {
				t.Errorf("Compare(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

// TestCompare_Antisymmetric checks that swapping the operands negates the
// result, the property the Upgrade/Downgrade distinction relies on.
func TestCompare_Antisymmetric(t *testing.T) {
	pairs := [][2]string{
		{"1.0.0", "1.0.1"},
		{"v6.4.2", "6.5.0"},
		{"5.4", "5.4.1"},
		{"1.0.0-rc.1", "1.0.0"},
		{"1.0.0-alpha", "1.0.0-alpha.1"},
	}

	for _, p := range pairs {
		t.Run(p[0]+" vs "+p[1], func(t *testing.T) {
			forward, okForward := Compare(p[0], p[1])
			backward, okBackward := Compare(p[1], p[0])

			if !okForward || !okBackward {
				t.Fatalf("Compare(%q, %q) expected both directions comparable, got %v and %v", p[0], p[1], okForward, okBackward)
			}

			if forward != -backward {
				t.Errorf("Compare(%q, %q) = %d but reversed = %d, want negation", p[0], p[1], forward, backward)
			}
		})
	}
}
