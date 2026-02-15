package version

import "testing"

func TestParse_Valid(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		compare SemVer
	}{
		{
			name:    "simple",
			input:   "1.2.3",
			want:    "1.2.3",
			compare: SemVer{Major: 1, Minor: 2, Patch: 3},
		},
		{
			name:    "with prefix",
			input:   "v2.0.1",
			want:    "2.0.1",
			compare: SemVer{Major: 2, Minor: 0, Patch: 1},
		},
		{
			name:    "prerelease and build",
			input:   "1.0.0-rc.1+exp.sha",
			want:    "1.0.0-rc.1+exp.sha",
			compare: SemVer{Major: 1, Minor: 0, Patch: 0, PreRelease: "rc.1", Build: "exp.sha"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(tt.input)
			if err != nil {
				t.Fatalf("Parse(%q) returned error: %v", tt.input, err)
			}

			if got.String() != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got.String())
			}

			if got.Compare(tt.compare) != 0 {
				t.Fatalf("expected %#v, got %#v", tt.compare, got)
			}
		})
	}
}

func TestParse_Invalid(t *testing.T) {
	tests := []string{
		"",
		"1",
		"1.0",
		"1.0.0.0",
		"01.0.0",
		"1.0.0-01",
		"v1.0",
	}

	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			if _, err := Parse(input); err == nil {
				t.Fatalf("expected parse error for %q", input)
			}
		})
	}
}

func TestSemVerCompare(t *testing.T) {
	tests := []struct {
		left  string
		right string
		want  int
	}{
		{left: "1.0.0", right: "1.0.0", want: 0},
		{left: "1.0.1", right: "1.0.0", want: 1},
		{left: "1.0.0", right: "1.0.1", want: -1},
		{left: "2.0.0", right: "1.9.9", want: 1},
		{left: "1.0.0-alpha", right: "1.0.0", want: -1},
		{left: "1.0.0-alpha.1", right: "1.0.0-alpha.beta", want: -1},
		{left: "1.0.0-beta", right: "1.0.0-alpha.1", want: 1},
		{left: "1.0.0+build.1", right: "1.0.0+build.2", want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.left+"_vs_"+tt.right, func(t *testing.T) {
			left := MustParse(tt.left)
			right := MustParse(tt.right)

			got := left.Compare(right)
			if got != tt.want {
				t.Fatalf("compare(%s, %s): expected %d, got %d", tt.left, tt.right, tt.want, got)
			}
		})
	}
}

func TestSemVerNext(t *testing.T) {
	v := MustParse("1.2.3-rc.1+meta")

	if got := v.NextPatch().String(); got != "1.2.4" {
		t.Fatalf("expected next patch 1.2.4, got %s", got)
	}

	if got := v.NextMinor().String(); got != "1.3.0" {
		t.Fatalf("expected next minor 1.3.0, got %s", got)
	}

	if got := v.NextMajor().String(); got != "2.0.0" {
		t.Fatalf("expected next major 2.0.0, got %s", got)
	}
}
