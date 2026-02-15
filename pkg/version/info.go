package version

import (
	"fmt"
	"strings"
	"time"
)

const (
	// Unknown is used when build metadata is not provided.
	Unknown = "unknown"
	// DevelopmentVersion is the default version in local builds.
	DevelopmentVersion = "dev"
)

var (
	// AppVersion is intended to be overridden at build time:
	// go build -ldflags="-X github.com/nimburion/nimburion/pkg/version.AppVersion=v1.2.3"
	AppVersion = DevelopmentVersion

	// GitCommit is intended to be overridden at build time.
	GitCommit = Unknown

	// BuildTime is intended to be overridden at build time (RFC3339 recommended).
	BuildTime = Unknown
)

// Info contains version metadata for an application.
type Info struct {
	Service   string `json:"service"`
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildTime string `json:"build_time"`
}

// Current returns the current build version metadata.
func Current(serviceName string) Info {
	return Info{
		Service:   normalizeOrDefault(serviceName, Unknown),
		Version:   normalizeOrDefault(AppVersion, DevelopmentVersion),
		Commit:    normalizeOrDefault(GitCommit, Unknown),
		BuildTime: normalizeOrDefault(BuildTime, Unknown),
	}
}

// ParseBuildTime parses BuildTime as RFC3339 if present.
func (i Info) ParseBuildTime() (time.Time, bool) {
	if i.BuildTime == "" || i.BuildTime == Unknown {
		return time.Time{}, false
	}

	ts, err := time.Parse(time.RFC3339, i.BuildTime)
	if err != nil {
		return time.Time{}, false
	}
	return ts, true
}

// SemVer parses the Info version if it is a semantic version.
func (i Info) SemVer() (SemVer, bool) {
	v, err := Parse(i.Version)
	if err != nil {
		return SemVer{}, false
	}
	return v, true
}

// String returns a log-friendly representation.
func (i Info) String() string {
	return fmt.Sprintf("%s@%s (commit=%s, build_time=%s)", i.Service, i.Version, i.Commit, i.BuildTime)
}

func normalizeOrDefault(v, fallback string) string {
	norm := strings.TrimSpace(v)
	if norm == "" {
		return fallback
	}
	return norm
}
