// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package docker

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	networkapi "github.com/docker/docker/api/types/network"
	dockerC "github.com/docker/docker/client"
	clabruntime "github.com/srl-labs/containerlab/runtime"
	clabtypes "github.com/srl-labs/containerlab/types"
)

// fakeDockerNetworkServer is a minimal stand-in for the docker daemon's
// /networks endpoints. While created==false, NetworkInspect returns 404 and
// NetworkCreate flips created==true; thereafter NetworkCreate returns 409
// Conflict and NetworkInspect serves the stored network. This mirrors the
// daemon's response when two callers race to create the same network.
type fakeDockerNetworkServer struct {
	t       *testing.T
	netName string
	mu      sync.Mutex
	created bool
	info    networkapi.Inspect
	creates atomic.Int32
}

func (f *fakeDockerNetworkServer) handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Docker client prefixes paths with /v<version>/; strip it.
		path := r.URL.Path
		if strings.HasPrefix(path, "/v") {
			if idx := strings.Index(path[1:], "/"); idx != -1 {
				path = path[idx+1:]
			}
		}

		switch {
		case r.Method == http.MethodPost && path == "/networks/create":
			f.creates.Add(1)

			var req networkapi.CreateRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			f.mu.Lock()
			if f.created {
				f.mu.Unlock()
				writeJSON(w, http.StatusConflict, map[string]string{
					"message": "network with name " + f.netName + " already exists",
				})
				return
			}

			f.info = networkapi.Inspect{
				ID:     "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789",
				Name:   req.Name,
				Driver: "bridge",
				Options: map[string]string{
					bridgeNameOption: "br-fake",
				},
			}
			f.created = true
			f.mu.Unlock()

			writeJSON(w, http.StatusCreated, networkapi.CreateResponse{ID: f.info.ID})

		case r.Method == http.MethodGet && strings.HasPrefix(path, "/networks/"):
			f.mu.Lock()
			created := f.created
			info := f.info
			f.mu.Unlock()

			if !created {
				writeJSON(w, http.StatusNotFound, map[string]string{
					"message": "network " + f.netName + " not found",
				})
				return
			}

			writeJSON(w, http.StatusOK, info)

		default:
			f.t.Logf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func newFakeDockerRuntime(t *testing.T, netName string) (*DockerRuntime, *fakeDockerNetworkServer, func()) {
	t.Helper()

	fake := &fakeDockerNetworkServer{t: t, netName: netName}
	srv := httptest.NewServer(fake.handler())

	cli, err := dockerC.NewClientWithOpts(
		dockerC.WithHost(srv.URL),
		dockerC.WithVersion("1.43"),
	)
	if err != nil {
		srv.Close()
		t.Fatal(err)
	}

	rt := &DockerRuntime{
		Client: cli,
		mgmt: &clabtypes.MgmtNet{
			Network:    netName,
			IPv4Subnet: "172.45.99.0/24",
			MTU:        1500,
		},
		config:  clabruntime.RuntimeConfig{Timeout: defaultTimeout},
		version: "v27.0.0",
	}

	return rt, fake, func() { _ = cli.Close(); srv.Close() }
}

// TestCreateMgmtBridge_ConcurrentSafe is the regression test for the race
// fixed by this PR: N goroutines all call createMgmtBridge for the same
// docker network. The first to call NetworkCreate wins, the rest receive
// 409 Conflict from the daemon and must recover via re-inspect rather than
// propagate the error.
//
// Without the errdefs.IsConflict branch the losing goroutines return
// "network with name clab already exists".
func TestCreateMgmtBridge_ConcurrentSafe(t *testing.T) {
	rt, fake, cleanup := newFakeDockerRuntime(t, "clab")
	defer cleanup()

	const n = 16

	start := make(chan struct{})

	var wg sync.WaitGroup

	names := make([]string, n)
	errs := make([]error, n)

	for i := 0; i < n; i++ {
		wg.Add(1)

		go func(i int) {
			defer wg.Done()
			<-start
			names[i], errs[i] = rt.createMgmtBridge(context.Background(), "")
		}(i)
	}

	close(start)
	wg.Wait()

	for i, e := range errs {
		if e != nil {
			t.Errorf("goroutine %d: %v", i, e)
		}
	}

	for i, got := range names {
		if got == "" {
			t.Errorf("goroutine %d returned empty bridge name", i)
		}
	}

	if got := fake.creates.Load(); int(got) < 2 {
		t.Errorf("expected concurrent goroutines to all attempt NetworkCreate, only saw %d", got)
	}
}
