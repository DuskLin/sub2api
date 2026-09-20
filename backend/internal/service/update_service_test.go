//go:build unit

package service

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type updateServiceCacheStub struct {
	data string
}

func (s *updateServiceCacheStub) GetUpdateInfo(context.Context) (string, error) {
	if s.data == "" {
		return "", errors.New("cache miss")
	}
	return s.data, nil
}

func (s *updateServiceCacheStub) SetUpdateInfo(_ context.Context, data string, _ time.Duration) error {
	s.data = data
	return nil
}

type updateServiceGitHubClientStub struct {
	release        *GitHubRelease
	recentReleases []*GitHubRelease
	recentErr      error
	requestedRepo  string
	downloadURL    string
	downloadErr    error
}

func (s *updateServiceGitHubClientStub) FetchLatestRelease(_ context.Context, repo string) (*GitHubRelease, error) {
	s.requestedRepo = repo
	return s.release, nil
}

func (s *updateServiceGitHubClientStub) FetchRecentReleases(_ context.Context, repo string, _ int) ([]*GitHubRelease, error) {
	s.requestedRepo = repo
	return s.recentReleases, s.recentErr
}

func TestUpdateLocalChannel(t *testing.T) {
	client := &updateServiceGitHubClientStub{recentReleases: []*GitHubRelease{
		{TagName: "v9.0.0"},
		{TagName: "v0.2.4-local.99", Draft: true},
		{TagName: "v0.2.4-local.2", Prerelease: true},
		{TagName: "v0.2.4-local.10", Prerelease: true},
		{TagName: "v0.2.4-local.1", Prerelease: true},
		{TagName: "v1.0.0-beta.1", Prerelease: true},
		{TagName: "v1.0.0-local.bad", Prerelease: true},
	}}
	cache := &updateServiceCacheStub{}
	svc := NewUpdateService(cache, client, "0.2.4-local.1", "release")
	info, err := svc.CheckUpdate(context.Background(), true)
	require.NoError(t, err)
	require.Empty(t, info.Warning)
	require.True(t, info.HasUpdate)
	require.Equal(t, "0.2.4-local.10", info.LatestVersion)
	require.Equal(t, localGitHubRepo, client.requestedRepo)
	require.Equal(t, "jlliu0204/sub2api-local", info.DockerImage)

	info, err = svc.CheckUpdate(context.Background(), false)
	require.NoError(t, err)
	require.True(t, info.Cached)
	require.True(t, info.HasUpdate)

	svc.currentVersion = "0.2.4-local.10"
	versions, err := svc.ListRollbackVersions(context.Background())
	require.NoError(t, err)
	require.Equal(t, []RollbackVersion{{Version: "0.2.4-local.2"}, {Version: "0.2.4-local.1"}}, versions)

	// Official cached data must never be offered to local installs (or vice versa).
	svc.currentVersion = "0.2.4"
	_, err = svc.getFromCache(context.Background())
	require.Error(t, err)
}

func TestUpdateLocalVersionsCompareNumerically(t *testing.T) {
	for _, pair := range [][2]string{
		{"0.2.4-local.1", "0.2.4-local.2"},
		{"0.2.4-local.2", "0.2.4-local.10"},
		{"0.2.4-local.10", "0.2.5-local.1"},
		{"v0.2.4-local.1", "v0.2.4-local.2"},
	} {
		require.Less(t, compareVersions(pair[0], pair[1]), 0)
		require.Greater(t, compareVersions(pair[1], pair[0]), 0)
		require.Zero(t, compareVersions(pair[0], pair[0]))
	}
}

func TestUpdateLocalChannelDoesNotFallBackToOfficial(t *testing.T) {
	client := &updateServiceGitHubClientStub{recentReleases: []*GitHubRelease{{TagName: "v9.0.0"}}}
	svc := NewUpdateService(&updateServiceCacheStub{}, client, "0.2.4-local.1", "release")
	info, err := svc.CheckUpdate(context.Background(), true)
	require.NoError(t, err)
	require.False(t, info.HasUpdate)
	require.Contains(t, info.Warning, "no published release")
	require.Equal(t, localGitHubRepo, info.Repository)
}

func TestUpdateDockerDownloadsLocalRelease(t *testing.T) {
	t.Setenv("SUB2API_DEPLOYMENT", "docker")
	// Stop at the download boundary: never replace the running test executable.
	stopDownload := errors.New("test download boundary")
	assetName := fmt.Sprintf("sub2api_0.2.7-local.1_%s_%s.tar.gz", runtime.GOOS, runtime.GOARCH)
	assetURL := "https://github.com/DuskLin/sub2api/releases/download/v0.2.7-local.1/" + assetName
	client := &updateServiceGitHubClientStub{
		downloadErr: stopDownload,
		recentReleases: []*GitHubRelease{
			{TagName: "v9.0.0"}, // Stable releases are outside the local channel.
			{TagName: "v0.2.7-local.1", Prerelease: true, Assets: []GitHubAsset{{Name: assetName, BrowserDownloadURL: assetURL}}},
		},
	}
	svc := NewUpdateService(&updateServiceCacheStub{}, client, "0.2.5-local.1", "release")
	require.ErrorIs(t, svc.PerformUpdate(context.Background()), stopDownload)
	require.Equal(t, localGitHubRepo, client.requestedRepo)
	require.Equal(t, assetURL, client.downloadURL)

	// Online rollback also works in Docker, but only to a listed older local version.
	client.downloadURL = ""
	svc.currentVersion = "0.2.7-local.2"
	require.ErrorIs(t, svc.RollbackToVersion(context.Background(), "0.2.7-local.1"), stopDownload)
	require.Equal(t, assetURL, client.downloadURL)
	client.downloadURL = ""
	require.ErrorIs(t, svc.RollbackToVersion(context.Background(), "9.0.0"), ErrRollbackVersionNotAllowed)
	require.Empty(t, client.downloadURL)

	// An up-to-date container should still get the normal no-update response.
	svc.currentVersion = "0.2.7-local.1"
	require.ErrorIs(t, svc.PerformUpdate(context.Background()), ErrNoUpdateAvailable)
}

func (s *updateServiceGitHubClientStub) DownloadFile(_ context.Context, url, _ string, _ int64) error {
	if s.downloadErr != nil {
		s.downloadURL = url
		return s.downloadErr
	}
	panic("DownloadFile should not be called when no update is available")
}

func (s *updateServiceGitHubClientStub) FetchChecksumFile(context.Context, string) ([]byte, error) {
	panic("FetchChecksumFile should not be called when no update is available")
}

func TestUpdateServicePerformUpdateNoUpdateReturnsSentinel(t *testing.T) {
	svc := NewUpdateService(
		&updateServiceCacheStub{},
		&updateServiceGitHubClientStub{
			release: &GitHubRelease{
				TagName: "v0.1.132",
				Name:    "v0.1.132",
			},
		},
		"0.1.132",
		"release",
	)

	err := svc.PerformUpdate(context.Background())

	require.Error(t, err)
	require.True(t, errors.Is(err, ErrNoUpdateAvailable))
	require.ErrorIs(t, err, ErrNoUpdateAvailable)
}

func newRollbackTestService(current string, releases []*GitHubRelease) *UpdateService {
	return NewUpdateService(
		&updateServiceCacheStub{},
		&updateServiceGitHubClientStub{recentReleases: releases},
		current,
		"release",
	)
}

func TestUpdateServiceListRollbackVersionsFiltersAndCaps(t *testing.T) {
	releases := []*GitHubRelease{
		{TagName: "v0.1.148", PublishedAt: "2026-07-09T00:00:00Z"},                       // newer than current: excluded
		{TagName: "v0.1.147", PublishedAt: "2026-07-08T00:00:00Z"},                       // current: excluded
		{TagName: "v0.1.146-rc1", PublishedAt: "2026-07-07T12:00:00Z", Prerelease: true}, // prerelease: excluded
		{TagName: "v0.1.146", PublishedAt: "2026-07-07T00:00:00Z"},
		{TagName: "v0.1.145", PublishedAt: "2026-07-06T00:00:00Z", Draft: true}, // draft: excluded
		{TagName: "v0.1.144", PublishedAt: "2026-07-05T00:00:00Z"},
		{TagName: "v0.1.144", PublishedAt: "2026-07-05T00:00:00Z"}, // duplicate: excluded
		{TagName: "v0.1.143", PublishedAt: "2026-07-04T00:00:00Z"},
		{TagName: "v0.1.142", PublishedAt: "2026-07-03T00:00:00Z"}, // beyond cap of 3: excluded
	}
	svc := newRollbackTestService("0.1.147", releases)

	versions, err := svc.ListRollbackVersions(context.Background())

	require.NoError(t, err)
	require.Len(t, versions, 3)
	require.Equal(t, "0.1.146", versions[0].Version)
	require.Equal(t, "0.1.144", versions[1].Version)
	require.Equal(t, "0.1.143", versions[2].Version)
}

func TestUpdateServiceListRollbackVersionsSortsUnorderedInput(t *testing.T) {
	releases := []*GitHubRelease{
		{TagName: "v0.1.144"},
		{TagName: "v0.1.146"},
		{TagName: "v0.1.145"},
	}
	svc := newRollbackTestService("0.1.147", releases)

	versions, err := svc.ListRollbackVersions(context.Background())

	require.NoError(t, err)
	require.Len(t, versions, 3)
	require.Equal(t, "0.1.146", versions[0].Version)
	require.Equal(t, "0.1.145", versions[1].Version)
	require.Equal(t, "0.1.144", versions[2].Version)
}

func TestUpdateServiceListRollbackVersionsEmptyWhenNoneOlder(t *testing.T) {
	releases := []*GitHubRelease{
		{TagName: "v0.1.147"},
		{TagName: "v0.1.148"},
	}
	svc := newRollbackTestService("0.1.147", releases)

	versions, err := svc.ListRollbackVersions(context.Background())

	require.NoError(t, err)
	require.Empty(t, versions)
}

func TestUpdateServiceListRollbackVersionsPropagatesFetchError(t *testing.T) {
	svc := NewUpdateService(
		&updateServiceCacheStub{},
		&updateServiceGitHubClientStub{recentErr: errors.New("github unavailable")},
		"0.1.147",
		"release",
	)

	_, err := svc.ListRollbackVersions(context.Background())

	require.Error(t, err)
	require.Contains(t, err.Error(), "github unavailable")
}

func TestUpdateServiceRollbackToVersionRejectsDisallowedTargets(t *testing.T) {
	releases := []*GitHubRelease{
		{TagName: "v0.1.148"},
		{TagName: "v0.1.147"},
		{TagName: "v0.1.146"},
		{TagName: "v0.1.145"},
		{TagName: "v0.1.144"},
		{TagName: "v0.1.143"},
		{TagName: "v0.1.142"},
	}
	svc := newRollbackTestService("0.1.147", releases)

	for _, target := range []string{
		"",         // empty
		"0.1.147",  // current version
		"v0.1.147", // current version with prefix
		"0.1.148",  // newer than current
		"0.1.142",  // older than the 3 most recent
		"9.9.9",    // nonexistent
	} {
		err := svc.RollbackToVersion(context.Background(), target)
		require.ErrorIs(t, err, ErrRollbackVersionNotAllowed, "target %q should be rejected", target)
	}
}

func TestUpdateServiceRollbackToVersionAcceptsVPrefix(t *testing.T) {
	// No platform asset in the release: the target passes the allowlist check
	// and fails later at asset lookup, proving the version itself was accepted.
	releases := []*GitHubRelease{
		{TagName: "v0.1.147"},
		{TagName: "v0.1.146"},
	}
	svc := newRollbackTestService("0.1.147", releases)

	err := svc.RollbackToVersion(context.Background(), "v0.1.146")

	require.Error(t, err)
	require.NotErrorIs(t, err, ErrRollbackVersionNotAllowed)
	require.Contains(t, err.Error(), "no compatible release found")
}
