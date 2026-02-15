package version

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var semVerPattern = regexp.MustCompile(`^v?(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?(?:\+([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?$`)

// SemVer represents a semantic version according to semver.org.
type SemVer struct {
	Major int64
	Minor int64
	Patch int64

	PreRelease string
	Build      string
}

// Parse parses a semantic version string.
func Parse(raw string) (SemVer, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return SemVer{}, errors.New("version cannot be empty")
	}

	matches := semVerPattern.FindStringSubmatch(raw)
	if len(matches) != 6 {
		return SemVer{}, fmt.Errorf("invalid semantic version: %q", raw)
	}

	major, err := strconv.ParseInt(matches[1], 10, 64)
	if err != nil {
		return SemVer{}, fmt.Errorf("invalid major version: %w", err)
	}
	minor, err := strconv.ParseInt(matches[2], 10, 64)
	if err != nil {
		return SemVer{}, fmt.Errorf("invalid minor version: %w", err)
	}
	patch, err := strconv.ParseInt(matches[3], 10, 64)
	if err != nil {
		return SemVer{}, fmt.Errorf("invalid patch version: %w", err)
	}

	v := SemVer{
		Major:      major,
		Minor:      minor,
		Patch:      patch,
		PreRelease: matches[4],
		Build:      matches[5],
	}

	if err := validatePreRelease(v.PreRelease); err != nil {
		return SemVer{}, err
	}

	return v, nil
}

// MustParse parses a semantic version string and panics on error.
func MustParse(raw string) SemVer {
	v, err := Parse(raw)
	if err != nil {
		panic(err)
	}
	return v
}

// IsValid reports whether a semantic version is valid.
func IsValid(raw string) bool {
	_, err := Parse(raw)
	return err == nil
}

// String returns the canonical string representation.
func (v SemVer) String() string {
	base := fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
	if v.PreRelease != "" {
		base += "-" + v.PreRelease
	}
	if v.Build != "" {
		base += "+" + v.Build
	}
	return base
}

// NextMajor returns the next major version.
func (v SemVer) NextMajor() SemVer {
	return SemVer{Major: v.Major + 1}
}

// NextMinor returns the next minor version.
func (v SemVer) NextMinor() SemVer {
	return SemVer{Major: v.Major, Minor: v.Minor + 1}
}

// NextPatch returns the next patch version.
func (v SemVer) NextPatch() SemVer {
	return SemVer{Major: v.Major, Minor: v.Minor, Patch: v.Patch + 1}
}

// Compare compares this version with another one.
// It returns -1 when v < other, 1 when v > other and 0 when equal.
func (v SemVer) Compare(other SemVer) int {
	if c := compareInt(v.Major, other.Major); c != 0 {
		return c
	}
	if c := compareInt(v.Minor, other.Minor); c != 0 {
		return c
	}
	if c := compareInt(v.Patch, other.Patch); c != 0 {
		return c
	}
	return comparePreRelease(v.PreRelease, other.PreRelease)
}

func compareInt(a, b int64) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

func comparePreRelease(a, b string) int {
	if a == b {
		return 0
	}
	if a == "" {
		return 1
	}
	if b == "" {
		return -1
	}

	aParts := strings.Split(a, ".")
	bParts := strings.Split(b, ".")

	for i := 0; i < len(aParts) && i < len(bParts); i++ {
		if aParts[i] == bParts[i] {
			continue
		}

		aNum, aNumErr := strconv.ParseInt(aParts[i], 10, 64)
		bNum, bNumErr := strconv.ParseInt(bParts[i], 10, 64)

		switch {
		case aNumErr == nil && bNumErr == nil:
			return compareInt(aNum, bNum)
		case aNumErr == nil && bNumErr != nil:
			return -1
		case aNumErr != nil && bNumErr == nil:
			return 1
		default:
			if aParts[i] < bParts[i] {
				return -1
			}
			return 1
		}
	}

	if len(aParts) < len(bParts) {
		return -1
	}
	return 1
}

func validatePreRelease(pr string) error {
	if pr == "" {
		return nil
	}

	ids := strings.Split(pr, ".")
	for _, id := range ids {
		if id == "" {
			return errors.New("invalid prerelease identifier: empty")
		}

		if _, err := strconv.ParseInt(id, 10, 64); err == nil && len(id) > 1 && id[0] == '0' {
			return fmt.Errorf("invalid prerelease numeric identifier %q: leading zero", id)
		}
	}
	return nil
}
