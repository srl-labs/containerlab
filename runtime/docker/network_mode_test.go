package docker

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	containerTypes "github.com/docker/docker/api/types/container"
	networkTypes "github.com/docker/docker/api/types/network"
	dockerC "github.com/docker/docker/client"
	clabruntime "github.com/srl-labs/containerlab/runtime"
	clabtypes "github.com/srl-labs/containerlab/types"
)

// TestProcessNetworkModeLabNamedAsNode covers a node whose name is also the lab name. The container
// it references must be looked up under the full lab prefix.
func TestProcessNetworkModeLabNamedAsNode(t *testing.T) {
	const holder = "clab-srl02-srl02-netns"
	id := strings.Repeat("a", 64)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		case strings.HasSuffix(r.URL.Path, "/containers/json"):
			if !strings.Contains(r.URL.Query().Get("filters"), "^"+holder+"$") {
				_, _ = w.Write([]byte(`[]`))
				return
			}
			_, _ = w.Write([]byte(
				`[{"Id":"` + id + `","Names":["/` + holder + `"],"NetworkSettings":{"Networks":{}}}]`,
			))
		case strings.Contains(r.URL.Path, id):
			_, _ = w.Write([]byte(`{"Id":"` + id + `","State":{"Pid":1234}}`))
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

	hostConfig := &containerTypes.HostConfig{}
	err = rt.processNetworkMode(
		context.Background(),
		&networkTypes.NetworkingConfig{},
		hostConfig,
		&containerTypes.Config{},
		&clabtypes.NodeConfig{
			LongName:    "clab-srl02-srl02",
			ShortName:   "srl02",
			NetworkMode: "container:srl02-netns",
		},
	)
	if err != nil {
		t.Fatalf("processNetworkMode() error = %v", err)
	}

	if want := containerTypes.NetworkMode("container:" + holder); hostConfig.NetworkMode != want {
		t.Fatalf("NetworkMode = %q, want %q", hostConfig.NetworkMode, want)
	}
}
