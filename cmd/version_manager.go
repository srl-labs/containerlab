package cmd

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/log"
	gover "github.com/hashicorp/go-version"
)

const (
	versionCheckInterval            = 100 * time.Millisecond
	expectedClabReleaseVersionParts = 2
)

var (
	versionManagerInstance     Manager   //nolint:gochecknoglobals
	versionManagerInstanceOnce sync.Once //nolint:gochecknoglobals
)

// InitManager initializes the version manager, we have an explicit init so we can capture the root
// context to ensure appropriate cancellation during the version fetching if necessary.
func initVersionManager(
	ctx context.Context,
) {
	versionManagerInstanceOnce.Do(func() {
		m := &manager{
			verLock:        sync.Mutex{},
			currentVersion: mustParseVersion(Version),
		}

		versionCheckStatus := os.Getenv("CLAB_VERSION_CHECK")

		log.Debugf("Env: CLAB_VERSION_CHECK=%s", versionCheckStatus)

		if strings.Contains(strings.ToLower(versionCheckStatus), "disable") {
			m.fetchDisabled = true
		}

		// run it so we check the version stuff in the background while user stuff is happening
		go m.run(ctx)

		versionManagerInstance = m
	})
}

func getVersionManager() Manager {
	if versionManagerInstance == nil {
		// be defensive, should be initialized during first cmd spin up though
		panic(
			"version manager instance is nil, this should not happen",
		)
	}

	return versionManagerInstance
}

// Manager is the config manager interface defining the version manager methods.
type Manager interface {
	GetLatestVersion(ctx context.Context) *gover.Version
	DisplayNewVersionAvailable(ctx context.Context)
}

type manager struct {
	fetchDisabled  bool
	verLock        sync.Mutex
	currentVersion *gover.Version
	latestVersion  *gover.Version
}

func (m *manager) GetLatestVersion(ctx context.Context) *gover.Version {
	if m.fetchDisabled {
		return nil
	}

	t := time.NewTicker(versionCheckInterval)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-t.C:
			m.verLock.Lock()

			latestVer := m.latestVersion

			// just locking so we can safely write it in, then reads are fine
			m.verLock.Unlock()

			if latestVer != nil {
				return latestVer
			}
		}
	}
}

func (m *manager) DisplayNewVersionAvailable(ctx context.Context) {
	latestVersion := m.GetLatestVersion(ctx)

	switch {
	case latestVersion == nil:
		fmt.Print("Failed fetching latest version information\n")
	case latestVersion.GreaterThan(m.currentVersion):
		printNewVersionInfo(latestVersion.String())
	default:
		fmt.Printf("You are on the latest version (%s)\n", Version)
	}
}

func (m *manager) run(ctx context.Context) {
	if m.fetchDisabled {
		return
	}

	client := &http.Client{
		// Don’t follow redirects
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	req, err := http.NewRequestWithContext(
		ctx, http.MethodHead,
		fmt.Sprintf("%s/releases/latest", repoUrl),
		http.NoBody,
	)
	if err != nil {
		log.Debugf("error occurred during latest version fetch: %v", err)

		return
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Debugf("error occurred during latest version fetch: %v", err)

		return
	}

	if resp == nil {
		log.Debug("no payload received during latest version fetch")

		return
	}

	defer func() {
		_ = resp.Body.Close()
	}()

	locationHeader := resp.Header.Get("Location")

	releaseParts := strings.Split(locationHeader, "releases/tag/")

	if len(releaseParts) != expectedClabReleaseVersionParts {
		log.Debugf("could not properly parse release version from %q", locationHeader)

		return
	}

	m.verLock.Lock()
	defer m.verLock.Unlock()

	m.latestVersion = mustParseVersion(releaseParts[1])
}

func mustParseVersion(v string) *gover.Version {
	parsed, err := gover.NewVersion(v)
	if err != nil {
		// swallow parsing errors and just log it
		log.Debugf("error occurred parsing version string %q, err: %v", v, err)

		return nil
	}

	return parsed
}
