// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package local

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	clabconstants "github.com/srl-labs/containerlab/constants"
	clabexec "github.com/srl-labs/containerlab/exec"
	clabinternaltuibackend "github.com/srl-labs/containerlab/internal/tui/backend"
	clabinternaltuitermtext "github.com/srl-labs/containerlab/internal/tui/termtext"
	clabruntime "github.com/srl-labs/containerlab/runtime"
)

const (
	// sshxLinkFile is where the sshx container stores the link of its session.
	sshxLinkFile = "/tmp/sshx"
	sshxLinkHost = "https://sshx.io/"

	// A fresh sshx container needs a moment to download the client and open the session.
	shareLinkAttempts = 10
	shareLinkRetry    = 3 * time.Second
	shareLinkTimeout  = 5 * time.Second
	// shareLinkRest is how long a container whose link could not be read is left alone.
	shareLinkRest = 2 * time.Minute
	// shareLinkClaim is how long a container is reserved for reading its link. It outlasts
	// the attempts, so that a reload of the inventory in between starts no second reader.
	shareLinkClaim = 2 * shareLinkAttempts * (shareLinkRetry + shareLinkTimeout)
)

// sshxLink holds the links of an sshx session.
type sshxLink struct {
	full     string
	readOnly string
}

// shareFromContainer maps a tool container to a terminal sharing session. Other tools are
// not sessions.
func shareFromContainer(
	ctr *clabruntime.GenericContainer,
	env *inventoryEnv,
) (clabinternaltuibackend.Share, bool) {
	share := clabinternaltuibackend.Share{Lab: ctr.Labels[clabconstants.Containerlab]}
	up := strings.EqualFold(ctr.State, stateRunning)

	switch clabinternaltuibackend.ShareTool(ctr.Labels[clabconstants.ToolType]) {
	case clabinternaltuibackend.ShareSSHX:
		share.Tool = clabinternaltuibackend.ShareSSHX

		if up {
			link := env.sshxLink(ctr.ID)
			share.Link, share.ReadOnlyLink = link.full, link.readOnly
		}
	case clabinternaltuibackend.ShareGoTTY:
		share.Tool = clabinternaltuibackend.ShareGoTTY

		if port := publishedPort(ctr); up && port != 0 {
			// The web terminal is published on the host, not on the management network.
			share.Link = fmt.Sprintf("http://%s:%d", env.hostname, port)
		}
	default:
		return share, false
	}

	return share, true
}

// publishedPort returns the first port of the container that is published on the host.
func publishedPort(ctr *clabruntime.GenericContainer) int {
	for _, p := range ctr.Ports {
		if p != nil && p.HostPort != 0 {
			return p.HostPort
		}
	}

	return 0
}

// parseSSHXLink extracts the session link from the content of the link file. With readers
// enabled the link carries two keys, of which the first one alone gives read-only access.
func parseSSHXLink(content string) (sshxLink, bool) {
	for field := range strings.FieldsSeq(content) {
		if !strings.HasPrefix(field, sshxLinkHost) ||
			clabinternaltuitermtext.Sanitize(field) != field {
			continue
		}

		link := sshxLink{full: field}

		if base, keys, found := strings.Cut(field, "#"); found {
			if reader, _, both := strings.Cut(keys, ","); both {
				link.readOnly = base + "#" + reader
			}
		}

		return link, true
	}

	return sshxLink{}, false
}

// shareLinks remembers the session links read from sshx containers. A link does not change
// while its container lives, and reading it takes a command run in the container.
type shareLinks struct {
	mu    sync.Mutex
	links map[string]sshxLink
	// busy holds the containers a link is being read from, or was read from in vain, with
	// the time they may be tried again.
	busy map[string]time.Time
}

func (s *shareLinks) link(containerID string) sshxLink {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.links[containerID]
}

// claim reserves a container for reading its link. It fails when the link is known, being
// read or was given up on recently.
func (s *shareLinks) claim(containerID string, now, until time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, known := s.links[containerID]; known {
		return false
	}

	if retryAt, ok := s.busy[containerID]; ok && now.Before(retryAt) {
		return false
	}

	if s.busy == nil {
		s.busy = map[string]time.Time{}
	}

	s.busy[containerID] = until

	return true
}

func (s *shareLinks) store(containerID string, link sshxLink) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.links == nil {
		s.links = map[string]sshxLink{}
	}

	s.links[containerID] = link
	delete(s.busy, containerID)
}

// giveUp leaves a container alone until the given time.
func (s *shareLinks) giveUp(containerID string, until time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.busy[containerID] = until
}

// retain forgets the containers that are gone.
func (s *shareLinks) retain(present map[string]struct{}) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for id := range s.links {
		if _, ok := present[id]; !ok {
			delete(s.links, id)
		}
	}

	for id := range s.busy {
		if _, ok := present[id]; !ok {
			delete(s.busy, id)
		}
	}
}

// resolveShareLinks starts reading the links of the sshx sessions that are not known yet.
func (b *Backend) resolveShareLinks(labs []*labRecord) {
	present := map[string]struct{}{}
	now := b.clock.Now()

	for _, rec := range labs {
		for _, tool := range rec.tools {
			present[tool.ID] = struct{}{}

			if tool.Labels[clabconstants.ToolType] !=
				string(clabinternaltuibackend.ShareSSHX) ||
				!strings.EqualFold(tool.State, stateRunning) {
				continue
			}

			if !b.shares.claim(tool.ID, now, now.Add(shareLinkClaim)) {
				continue
			}

			id, name := tool.ID, containerName(tool)

			b.spawn("share link reader", func() { b.readShareLink(b.ctx, id, name) })
		}
	}

	b.shares.retain(present)
}

// readShareLink reads the session link of an sshx container, trying until the session is
// up. The inventory is reloaded once the link is known.
func (b *Backend) readShareLink(ctx context.Context, id, container string) {
	for range shareLinkAttempts {
		if link, ok := b.fetchSSHXLink(ctx, container); ok {
			b.shares.store(id, link)
			b.requestReload()

			return
		}

		if !sleep(ctx, shareLinkRetry) {
			return
		}
	}

	b.shares.giveUp(id, b.clock.Now().Add(shareLinkRest))
	b.logf(levelWarn, "could not read the sshx link of %s", container)
}

func (b *Backend) fetchSSHXLink(ctx context.Context, container string) (sshxLink, bool) {
	ctx, cancel := context.WithTimeout(ctx, shareLinkTimeout)
	defer cancel()

	result, err := b.rt.Exec(ctx, container, clabexec.NewExecCmdFromSlice(
		[]string{"cat", sshxLinkFile}))
	if err != nil || result.GetReturnCode() != 0 {
		return sshxLink{}, false
	}

	return parseSSHXLink(result.GetStdOutString())
}
