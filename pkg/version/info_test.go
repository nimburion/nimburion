package version_test

import (
	"testing"
	"time"

	"github.com/nimburion/nimburion/pkg/version"
)

func TestCurrent_Defaults(t *testing.T) {
	oldVersion := version.AppVersion
	oldCommit := version.GitCommit
	oldBuildTime := version.BuildTime
	t.Cleanup(func() {
		version.AppVersion = oldVersion
		version.GitCommit = oldCommit
		version.BuildTime = oldBuildTime
	})

	version.AppVersion = ""
	version.GitCommit = ""
	version.BuildTime = ""

	info := version.Current("")

	if info.Service != version.Unknown {
		t.Fatalf("expected service %q, got %q", version.Unknown, info.Service)
	}
	if info.Version != version.DevelopmentVersion {
		t.Fatalf("expected version %q, got %q", version.DevelopmentVersion, info.Version)
	}
	if info.Commit != version.Unknown {
		t.Fatalf("expected commit %q, got %q", version.Unknown, info.Commit)
	}
	if info.BuildTime != version.Unknown {
		t.Fatalf("expected build_time %q, got %q", version.Unknown, info.BuildTime)
	}
}

func TestInfo_SemVer(t *testing.T) {
	info := version.Info{
		Service: "orders",
		Version: "v1.4.0",
	}

	v, ok := info.SemVer()
	if !ok {
		t.Fatalf("expected semantic version to be parsed")
	}
	if v.String() != "1.4.0" {
		t.Fatalf("expected 1.4.0, got %s", v.String())
	}
}

func TestInfo_ParseBuildTime(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	info := version.Info{
		BuildTime: now.Format(time.RFC3339),
	}

	parsed, ok := info.ParseBuildTime()
	if !ok {
		t.Fatalf("expected build time to be parsed")
	}
	if !parsed.Equal(now) {
		t.Fatalf("expected %s, got %s", now, parsed)
	}
}
