// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package docker

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	dockerTypes "github.com/docker/docker/api/types"
	containerTypes "github.com/docker/docker/api/types/container"
	dockerC "github.com/docker/docker/client"
	clabruntime "github.com/srl-labs/containerlab/runtime"
	clabtypes "github.com/srl-labs/containerlab/types"
)

// TestProduceGenericContainerList_SkipsConcurrentlyRemoved asserts the
// enumeration loop tolerates a 404 from ContainerInspect when a concurrent
// clab destroy removed the container between List and Inspect.
func TestProduceGenericContainerList_SkipsConcurrentlyRemoved(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"No such container"}`))
	}))
	defer srv.Close()

	cli, err := dockerC.NewClientWithOpts(
		dockerC.WithHost(srv.URL),
		dockerC.WithVersion("1.43"),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer cli.Close()

	rt := &DockerRuntime{
		Client: cli,
		mgmt:   &clabtypes.MgmtNet{Network: "clab"},
		config: clabruntime.RuntimeConfig{Timeout: defaultTimeout},
	}

	inputs := []dockerTypes.Container{
		{ID: strings.Repeat("a", 64), Names: []string{"/clab-x-n1"}},
		{ID: strings.Repeat("b", 64), Names: []string{"/clab-x-n2"}},
	}

	got, err := rt.produceGenericContainerList(context.Background(), inputs, nil)
	if err != nil {
		t.Fatalf("got error: %v", err)
	}

	if len(got) != 0 {
		t.Errorf("got %d surviving containers, want 0", len(got))
	}
}

// TestProduceGenericContainerList_MixedKeepsSurvivor asserts that when one
// container 404s and another inspects successfully, the survivor is returned.
func TestProduceGenericContainerList_MixedKeepsSurvivor(t *testing.T) {
	goneID := strings.Repeat("a", 64)
	survivorID := strings.Repeat("b", 64)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		case strings.Contains(r.URL.Path, goneID):
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"message":"No such container"}`))
		case strings.Contains(r.URL.Path, survivorID):
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"Id":"` + survivorID + `","State":{"Pid":1234}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	cli, err := dockerC.NewClientWithOpts(
		dockerC.WithHost(srv.URL),
		dockerC.WithVersion("1.43"),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer cli.Close()

	rt := &DockerRuntime{
		Client: cli,
		mgmt:   &clabtypes.MgmtNet{Network: "clab"},
		config: clabruntime.RuntimeConfig{Timeout: defaultTimeout},
	}

	inputs := []dockerTypes.Container{
		{
			ID:              goneID,
			Names:           []string{"/clab-x-gone"},
			NetworkSettings: &containerTypes.NetworkSettingsSummary{},
		},
		{
			ID:              survivorID,
			Names:           []string{"/clab-x-alive"},
			NetworkSettings: &containerTypes.NetworkSettingsSummary{},
		},
	}

	got, err := rt.produceGenericContainerList(context.Background(), inputs, nil)
	if err != nil {
		t.Fatalf("got error: %v", err)
	}

	if len(got) != 1 {
		t.Fatalf("got %d surviving containers, want 1", len(got))
	}

	if got[0].ID != survivorID {
		t.Errorf("survivor id = %q, want %q", got[0].ID, survivorID)
	}

	if got[0].Pid != 1234 {
		t.Errorf("survivor pid = %d, want 1234", got[0].Pid)
	}
}
