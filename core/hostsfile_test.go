// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package core

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	clabmocksmocknodes "github.com/srl-labs/containerlab/mocks/mocknodes"
	clabnodes "github.com/srl-labs/containerlab/nodes"
	clabtypes "github.com/srl-labs/containerlab/types"
	"go.uber.org/mock/gomock"
)

func TestFilteredApplyHostsArtifactsExcludeAbsentUnselectedNodes(t *testing.T) {
	withTestHostsFiles(t, t.TempDir())

	ctrl := gomock.NewController(t)
	live := clabmocksmocknodes.NewMockNode(ctrl)
	absent := clabmocksmocknodes.NewMockNode(ctrl)
	live.EXPECT().GetHostsEntries(gomock.Any()).Return(clabtypes.HostEntries{
		clabtypes.NewHostEntry("192.0.2.10", "selected", clabtypes.IpVersionV4),
	}, nil)

	c := &CLab{
		Config: &Config{Name: "filtered-artifacts"},
		Nodes: map[string]clabnodes.Node{
			"selected": live,
			"absent":   absent,
		},
	}
	artifactLab := c.applyArtifactLab(map[string]*runtimeNodeGroup{"selected": {}})
	if err := artifactLab.appendHostsFileEntries(context.Background()); err != nil {
		t.Fatal(err)
	}

	contents, err := os.ReadFile(clabHostsFilename)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(contents), "192.0.2.10\tselected") {
		t.Fatalf("selected live node missing from hosts artifacts:\n%s", contents)
	}
	if len(artifactLab.Nodes) != 1 || artifactLab.Nodes["selected"] != live {
		t.Fatalf("artifact node scope = %#v, want selected live node only", artifactLab.Nodes)
	}
	if len(c.Nodes) != 2 || c.Nodes["absent"] != absent {
		t.Fatal("filtered artifact scoping mutated the full desired node map")
	}
}

// withTestHostsFiles redirects the package hosts/lock paths to files in tmp
// and seeds a baseline hosts file.
func withTestHostsFiles(t *testing.T, tmp string) {
	t.Helper()

	origFile, origLock := clabHostsFilename, clabHostsLockPath

	clabHostsFilename = filepath.Join(tmp, "hosts")
	clabHostsLockPath = filepath.Join(tmp, "hosts.lock")

	t.Cleanup(func() {
		clabHostsFilename = origFile
		clabHostsLockPath = origLock
	})

	if err := os.WriteFile(clabHostsFilename, []byte("127.0.0.1\tlocalhost\n"), 0o644); err != nil {
		t.Fatalf("seeding test hosts file: %v", err)
	}
}

// TestHostsFileConcurrentDeployAppend exercises appendHostsFileEntries from
// many goroutines and asserts that each lab ends up with exactly one block
// and the baseline lines are preserved.
func TestHostsFileConcurrentDeployAppend(t *testing.T) {
	withTestHostsFiles(t, t.TempDir())

	const (
		nLabs  = 16
		rounds = 25
	)

	labs := make([]*CLab, nLabs)
	for i := 0; i < nLabs; i++ {
		labs[i] = &CLab{
			Config: &Config{Name: fmt.Sprintf("race-deploy-%d", i)},
			Nodes:  map[string]clabnodes.Node{},
		}
	}

	start := make(chan struct{})

	var wg sync.WaitGroup

	errCh := make(chan error, nLabs*rounds)

	for _, lab := range labs {
		wg.Add(1)

		go func(c *CLab) {
			defer wg.Done()

			<-start

			for r := 0; r < rounds; r++ {
				if err := c.appendHostsFileEntries(context.Background()); err != nil {
					errCh <- fmt.Errorf("append %s round %d: %w", c.Config.Name, r, err)
					return
				}
			}
		}(lab)
	}

	close(start)
	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Error(err)
	}

	data, err := os.ReadFile(clabHostsFilename)
	if err != nil {
		t.Fatal(err)
	}

	contents := string(data)

	for _, lab := range labs {
		prefix := fmt.Sprintf(clabHostEntryPrefix, lab.Config.Name)
		postfix := fmt.Sprintf(clabHostEntryPostfix, lab.Config.Name)

		if got := strings.Count(contents, prefix); got != 1 {
			t.Errorf("lab %q: expected exactly 1 START marker, got %d", lab.Config.Name, got)
		}

		if got := strings.Count(contents, postfix); got != 1 {
			t.Errorf("lab %q: expected exactly 1 END marker, got %d", lab.Config.Name, got)
		}
	}

	if !strings.HasPrefix(contents, "127.0.0.1\tlocalhost\n") {
		t.Errorf("baseline localhost line was clobbered; file starts with %q",
			contents[:min(40, len(contents))])
	}
}

// TestHostsFileConcurrentDeployAndDestroy interleaves appends and removals
// from many goroutines; the file must return to its baseline with no
// CLAB-* markers left over.
func TestHostsFileConcurrentDeployAndDestroy(t *testing.T) {
	withTestHostsFiles(t, t.TempDir())

	const (
		nLabs  = 16
		rounds = 25
	)

	labs := make([]*CLab, nLabs)
	for i := 0; i < nLabs; i++ {
		labs[i] = &CLab{
			Config: &Config{Name: fmt.Sprintf("race-cycle-%d", i)},
			Nodes:  map[string]clabnodes.Node{},
		}
	}

	start := make(chan struct{})

	var wg sync.WaitGroup

	errCh := make(chan error, nLabs*rounds*2)

	for _, lab := range labs {
		wg.Add(1)

		go func(c *CLab) {
			defer wg.Done()

			<-start

			for r := 0; r < rounds; r++ {
				if err := c.appendHostsFileEntries(context.Background()); err != nil {
					errCh <- fmt.Errorf("append %s round %d: %w", c.Config.Name, r, err)
					return
				}

				if err := c.DeleteEntriesFromHostsFile(); err != nil {
					errCh <- fmt.Errorf("delete %s round %d: %w", c.Config.Name, r, err)
					return
				}
			}
		}(lab)
	}

	close(start)
	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Error(err)
	}

	data, err := os.ReadFile(clabHostsFilename)
	if err != nil {
		t.Fatal(err)
	}

	contents := string(data)
	if strings.Contains(contents, "CLAB-") {
		t.Errorf("expected no CLAB-* markers after deploy/destroy cycles, got:\n%s", contents)
	}

	if got := strings.TrimSpace(contents); got != "127.0.0.1\tlocalhost" {
		t.Errorf("expected baseline preserved, got %q", got)
	}
}
