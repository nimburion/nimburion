package version

import (
	"testing"
	"time"
)

func TestCurrent_Defaults(t *testing.T) {
	oldVersion := AppVersion
	oldCommit := GitCommit
	oldBuildTime := BuildTime
	t.Cleanup(func() {
		AppVersion = oldVersion
		GitCommit = oldCommit
		BuildTime = oldBuildTime
	})

	AppVersion = ""
	GitCommit = ""
	BuildTime = ""

	info := Current("")

	if info.Service != Unknown {
		t.Fatalf("expected service %q, got %q", Unknown, info.Service)
	}
	if info.Version != DevelopmentVersion {
		t.Fatalf("expected version %q, got %q", DevelopmentVersion, info.Version)
	}
	if info.Commit != Unknown {
		t.Fatalf("expected commit %q, got %q", Unknown, info.Commit)
	}
	if info.BuildTime != Unknown {
		t.Fatalf("expected build_time %q, got %q", Unknown, info.BuildTime)
	}
}

func TestInfo_SemVer(t *testing.T) {
	info := Info{
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
	info := Info{
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

