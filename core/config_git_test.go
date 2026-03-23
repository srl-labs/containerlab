// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	clabconstants "github.com/srl-labs/containerlab/constants"
	clabtypes "github.com/srl-labs/containerlab/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestGitRepo creates a temporary Git repository for testing.
func setupTestGitRepo(t *testing.T, branchName string) (string, string, func()) {
	t.Helper()

	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "clab-git-test-*")
	require.NoError(t, err)

	cleanup := func() {
		os.RemoveAll(tmpDir)
	}

	// Initialize Git repository
	repo, err := gogit.PlainInit(tmpDir, false)
	require.NoError(t, err)

	// Create a test file
	testFile := filepath.Join(tmpDir, "test.txt")
	err = os.WriteFile(testFile, []byte("test content"), 0644)
	require.NoError(t, err)

	// Get worktree
	wt, err := repo.Worktree()
	require.NoError(t, err)

	// Add file
	_, err = wt.Add("test.txt")
	require.NoError(t, err)

	// Create commit
	commit, err := wt.Commit("Initial commit", &gogit.CommitOptions{
		Author: &object.Signature{
			Name:  "Test User",
			Email: "test@example.com",
		},
	})
	require.NoError(t, err)

	// Get commit hash (short version - 7 characters)
	commitHash := commit.String()[:7]

	// Create and checkout branch if specified
	if branchName != "" {
		// Create branch reference
		branchRef := plumbing.NewBranchReferenceName(branchName)
		err = repo.Storer.SetReference(plumbing.NewHashReference(branchRef, commit))
		require.NoError(t, err)

		// Checkout branch
		err = wt.Checkout(&gogit.CheckoutOptions{
			Branch: branchRef,
			Create: false,
		})
		require.NoError(t, err)
	}

	return tmpDir, commitHash, cleanup
}

func TestGetGitInfo_WithGitRepo(t *testing.T) {
	tests := []struct {
		name       string
		branchName string
		wantBranch string
		wantHash   bool // true if we want a non-empty hash
	}{
		{
			name:       "simple_branch",
			branchName: "main",
			wantBranch: "main",
			wantHash:   true,
		},
		{
			name:       "feature_branch",
			branchName: "feature/test",
			wantBranch: "feature/test",
			wantHash:   true,
		},
		{
			name:       "branch_with_slashes",
			branchName: "feature/123/test-branch",
			wantBranch: "feature/123/test-branch",
			wantHash:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repoDir, expectedHash, cleanup := setupTestGitRepo(t, tt.branchName)
			defer cleanup()

			// Create a dummy topology file in the repo for TopoPaths
			dummyTopoFile := filepath.Join(repoDir, "dummy.clab.yml")
			err := os.WriteFile(dummyTopoFile, []byte("name: test"), 0644)
			require.NoError(t, err)

			c := &CLab{
				TopoPaths: &clabtypes.TopoPaths{},
			}
			// Set the topology file path so TopologyFileDir() returns the repo directory
			err = c.TopoPaths.SetTopologyFilePath(dummyTopoFile)
			require.NoError(t, err)

			branch, hash := c.getGitInfo()

			assert.Equal(t, tt.wantBranch, branch)
			if tt.wantHash {
				assert.NotEmpty(t, hash)
				assert.Equal(t, expectedHash, hash)
			}
			// Verify caching
			assert.NotEmpty(t, c.gitBranch)
			assert.Equal(t, tt.wantBranch, c.gitBranch)
			assert.Equal(t, expectedHash, c.gitHash)

			// Call again to verify caching works
			branch2, hash2 := c.getGitInfo()
			assert.Equal(t, branch, branch2)
			assert.Equal(t, hash, hash2)
		})
	}
}

func TestGetGitInfo_WithoutGitRepo(t *testing.T) {
	// Create temporary directory without Git repo
	tmpDir, err := os.MkdirTemp("", "clab-no-git-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create a dummy topology file for TopoPaths
	dummyTopoFile := filepath.Join(tmpDir, "dummy.clab.yml")
	err = os.WriteFile(dummyTopoFile, []byte("name: test"), 0644)
	require.NoError(t, err)

	c := &CLab{
		TopoPaths: &clabtypes.TopoPaths{},
	}
	err = c.TopoPaths.SetTopologyFilePath(dummyTopoFile)
	require.NoError(t, err)

	branch, hash := c.getGitInfo()

	assert.Equal(t, "none", branch)
	assert.Equal(t, "none", hash)
	assert.NotEmpty(t, c.gitBranch) // Should be cached even if "none"
	assert.Equal(t, "none", c.gitBranch)
	assert.Equal(t, "none", c.gitHash)
}

func TestMagicTopoNameReplacer_WithGitRepo(t *testing.T) {
	repoDir, commitHash, cleanup := setupTestGitRepo(t, "feature/test-branch")
	defer cleanup()

	// Create a dummy topology file in the repo for TopoPaths
	dummyTopoFile := filepath.Join(repoDir, "dummy.clab.yml")
	err := os.WriteFile(dummyTopoFile, []byte("name: test"), 0644)
	require.NoError(t, err)

	c := &CLab{
		Config: &Config{
			Name: "lab-__gitBranch__-__gitHash__",
		},
		TopoPaths: &clabtypes.TopoPaths{},
	}
	err = c.TopoPaths.SetTopologyFilePath(dummyTopoFile)
	require.NoError(t, err)

	replacer := c.magicTopoNameReplacer()
	result := replacer.Replace(c.Config.Name)

	// Branch name should have slashes replaced with hyphens
	expectedBranch := "feature-test-branch"
	assert.Contains(t, result, expectedBranch)
	assert.Contains(t, result, commitHash)
	assert.NotContains(t, result, "__gitBranch__")
	assert.NotContains(t, result, "__gitHash__")
}

func TestMagicTopoNameReplacer_WithoutGitRepo(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "clab-no-git-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create a dummy topology file for TopoPaths
	dummyTopoFile := filepath.Join(tmpDir, "dummy.clab.yml")
	err = os.WriteFile(dummyTopoFile, []byte("name: test"), 0644)
	require.NoError(t, err)

	c := &CLab{
		Config: &Config{
			Name: "lab-__gitBranch__-__gitHash__",
		},
		TopoPaths: &clabtypes.TopoPaths{},
	}
	err = c.TopoPaths.SetTopologyFilePath(dummyTopoFile)
	require.NoError(t, err)

	replacer := c.magicTopoNameReplacer()
	result := replacer.Replace(c.Config.Name)

	assert.Equal(t, "lab-none-none", result)
}

func TestMagicTopoNameReplacer_BranchNameSanitization(t *testing.T) {
	tests := []struct {
		name       string
		branchName string
		wantInName string
	}{
		{
			name:       "slash_replacement",
			branchName: "feature/test",
			wantInName: "feature-test",
		},
		{
			name:       "multiple_slashes",
			branchName: "feature/123/test",
			wantInName: "feature-123-test",
		},
		{
			name:       "no_slashes",
			branchName: "main",
			wantInName: "main",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repoDir, _, cleanup := setupTestGitRepo(t, tt.branchName)
			defer cleanup()

			// Create a dummy topology file in the repo for TopoPaths
			dummyTopoFile := filepath.Join(repoDir, "dummy.clab.yml")
			err := os.WriteFile(dummyTopoFile, []byte("name: test"), 0644)
			require.NoError(t, err)

			c := &CLab{
				Config: &Config{
					Name: "lab-__gitBranch__",
				},
				TopoPaths: &clabtypes.TopoPaths{},
			}
			err = c.TopoPaths.SetTopologyFilePath(dummyTopoFile)
			require.NoError(t, err)

			replacer := c.magicTopoNameReplacer()
			result := replacer.Replace(c.Config.Name)

			assert.Contains(t, result, tt.wantInName)
			assert.NotContains(t, result, "/")
		})
	}
}

func TestParseTopology_GitVariableSubstitution(t *testing.T) {
	repoDir, commitHash, cleanup := setupTestGitRepo(t, "feature/test")
	defer cleanup()

	// Create a dummy topology file in the repo for TopoPaths
	dummyTopoFile := filepath.Join(repoDir, "dummy.clab.yml")
	err := os.WriteFile(dummyTopoFile, []byte("name: test"), 0644)
	require.NoError(t, err)

	c := &CLab{
		Config: &Config{
			Name: "lab-__gitBranch__-__gitHash__",
		},
		TopoPaths: &clabtypes.TopoPaths{},
	}

	// Set topology file path
	err = c.TopoPaths.SetTopologyFilePath(dummyTopoFile)
	require.NoError(t, err)

	// Simulate the check in parseTopology() that triggers substitution
	if strings.Contains(c.Config.Name, gitBranchVar) ||
		strings.Contains(c.Config.Name, gitHashVar) {
		r := c.magicTopoNameReplacer()
		oldName := c.Config.Name
		c.Config.Name = r.Replace(c.Config.Name)
		assert.NotEqual(t, oldName, c.Config.Name)
	}

	// Verify that the name was substituted
	expectedName := "lab-feature-test-" + commitHash
	assert.Equal(t, expectedName, c.Config.Name)
}

func TestAddDefaultLabels_GitLabels(t *testing.T) {
	repoDir, commitHash, cleanup := setupTestGitRepo(t, "main")
	defer cleanup()

	// Create a dummy topology file in the repo for TopoPaths
	dummyTopoFile := filepath.Join(repoDir, "dummy.clab.yml")
	err := os.WriteFile(dummyTopoFile, []byte("name: test"), 0644)
	require.NoError(t, err)

	c := &CLab{
		Config: &Config{
			Name: "test-lab",
		},
		TopoPaths: &clabtypes.TopoPaths{},
	}
	err = c.TopoPaths.SetTopologyFilePath(dummyTopoFile)
	require.NoError(t, err)

	cfg := &clabtypes.NodeConfig{
		ShortName: "node1",
		LongName:  "clab-test-lab-node1",
		Kind:      "nokia_srlinux",
		LabDir:    "/tmp/clab-test-lab/node1",
	}

	c.addDefaultLabels(cfg)

	// Verify Git labels are present
	assert.Equal(t, "main", cfg.Labels[clabconstants.GitBranch])
	assert.Equal(t, commitHash, cfg.Labels[clabconstants.GitHash])
}

func TestAddDefaultLabels_NoGitRepo(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "clab-no-git-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create a dummy topology file for TopoPaths
	dummyTopoFile := filepath.Join(tmpDir, "dummy.clab.yml")
	err = os.WriteFile(dummyTopoFile, []byte("name: test"), 0644)
	require.NoError(t, err)

	c := &CLab{
		Config: &Config{
			Name: "test-lab",
		},
		TopoPaths: &clabtypes.TopoPaths{},
	}
	err = c.TopoPaths.SetTopologyFilePath(dummyTopoFile)
	require.NoError(t, err)

	cfg := &clabtypes.NodeConfig{
		ShortName: "node1",
		LongName:  "clab-test-lab-node1",
		Kind:      "nokia_srlinux",
		LabDir:    "/tmp/clab-test-lab/node1",
	}

	c.addDefaultLabels(cfg)

	// Verify Git labels are not present when not in a Git repo
	_, hasBranch := cfg.Labels[clabconstants.GitBranch]
	_, hasHash := cfg.Labels[clabconstants.GitHash]
	assert.False(t, hasBranch)
	assert.False(t, hasHash)
}

func TestGetGitInfo_Caching(t *testing.T) {
	repoDir, commitHash, cleanup := setupTestGitRepo(t, "main")
	defer cleanup()

	// Create a dummy topology file in the repo for TopoPaths
	dummyTopoFile := filepath.Join(repoDir, "dummy.clab.yml")
	err := os.WriteFile(dummyTopoFile, []byte("name: test"), 0644)
	require.NoError(t, err)

	c := &CLab{
		TopoPaths: &clabtypes.TopoPaths{},
	}
	err = c.TopoPaths.SetTopologyFilePath(dummyTopoFile)
	require.NoError(t, err)

	// First call should populate cache
	branch1, hash1 := c.getGitInfo()
	assert.NotEmpty(t, c.gitBranch) // Should be cached
	assert.Equal(t, "main", branch1)
	assert.Equal(t, commitHash, hash1)

	// Second call should use cache
	branch2, hash2 := c.getGitInfo()
	assert.Equal(t, branch1, branch2)
	assert.Equal(t, hash1, hash2)

	// Verify cache fields
	assert.Equal(t, "main", c.gitBranch)
	assert.Equal(t, commitHash, c.gitHash)
}

func TestMagicTopoNameReplacer_MultipleVariables(t *testing.T) {
	repoDir, commitHash, cleanup := setupTestGitRepo(t, "feature/test")
	defer cleanup()

	// Create a dummy topology file in the repo for TopoPaths
	dummyTopoFile := filepath.Join(repoDir, "dummy.clab.yml")
	err := os.WriteFile(dummyTopoFile, []byte("name: test"), 0644)
	require.NoError(t, err)

	c := &CLab{
		Config: &Config{
			Name: "__gitBranch__-__gitHash__-suffix",
		},
		TopoPaths: &clabtypes.TopoPaths{},
	}
	err = c.TopoPaths.SetTopologyFilePath(dummyTopoFile)
	require.NoError(t, err)

	replacer := c.magicTopoNameReplacer()
	result := replacer.Replace(c.Config.Name)

	assert.Contains(t, result, "feature-test")
	assert.Contains(t, result, commitHash)
	assert.Contains(t, result, "suffix")
	assert.NotContains(t, result, "__gitBranch__")
	assert.NotContains(t, result, "__gitHash__")
}

func TestMagicTopoNameReplacer_OnlyBranch(t *testing.T) {
	repoDir, _, cleanup := setupTestGitRepo(t, "main")
	defer cleanup()

	// Create a dummy topology file in the repo for TopoPaths
	dummyTopoFile := filepath.Join(repoDir, "dummy.clab.yml")
	err := os.WriteFile(dummyTopoFile, []byte("name: test"), 0644)
	require.NoError(t, err)

	c := &CLab{
		Config: &Config{
			Name: "lab-__gitBranch__",
		},
		TopoPaths: &clabtypes.TopoPaths{},
	}
	err = c.TopoPaths.SetTopologyFilePath(dummyTopoFile)
	require.NoError(t, err)

	replacer := c.magicTopoNameReplacer()
	result := replacer.Replace(c.Config.Name)

	assert.Equal(t, "lab-main", result)
}

func TestMagicTopoNameReplacer_OnlyHash(t *testing.T) {
	repoDir, commitHash, cleanup := setupTestGitRepo(t, "main")
	defer cleanup()

	// Create a dummy topology file in the repo for TopoPaths
	dummyTopoFile := filepath.Join(repoDir, "dummy.clab.yml")
	err := os.WriteFile(dummyTopoFile, []byte("name: test"), 0644)
	require.NoError(t, err)

	c := &CLab{
		Config: &Config{
			Name: "lab-__gitHash__",
		},
		TopoPaths: &clabtypes.TopoPaths{},
	}
	err = c.TopoPaths.SetTopologyFilePath(dummyTopoFile)
	require.NoError(t, err)

	replacer := c.magicTopoNameReplacer()
	result := replacer.Replace(c.Config.Name)

	assert.Equal(t, "lab-"+commitHash, result)
}

func TestParseTopology_GitVariableInName(t *testing.T) {
	repoDir, commitHash, cleanup := setupTestGitRepo(t, "feature/test")
	defer cleanup()

	// Create a dummy topology file in the repo for TopoPaths
	dummyTopoFile := filepath.Join(repoDir, "dummy.clab.yml")
	err := os.WriteFile(dummyTopoFile, []byte("name: test"), 0644)
	require.NoError(t, err)

	c := &CLab{
		Config: &Config{
			Name:     "lab-__gitBranch__",
			Topology: clabtypes.NewTopology(),
		},
		TopoPaths: &clabtypes.TopoPaths{},
	}
	err = c.TopoPaths.SetTopologyFilePath(dummyTopoFile)
	require.NoError(t, err)

	// Simulate the check in parseTopology
	if strings.Contains(c.Config.Name, gitBranchVar) ||
		strings.Contains(c.Config.Name, gitHashVar) {
		r := c.magicTopoNameReplacer()
		oldName := c.Config.Name
		c.Config.Name = r.Replace(c.Config.Name)
		assert.NotEqual(t, oldName, c.Config.Name)
		assert.Contains(t, c.Config.Name, "feature-test")
		assert.NotContains(t, c.Config.Name, "__gitBranch__")
	}

	// Verify Git info was cached
	assert.NotEmpty(t, c.gitBranch) // Should be cached
	assert.Equal(t, "feature/test", c.gitBranch)
	assert.Equal(t, commitHash, c.gitHash)
}
