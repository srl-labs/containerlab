package git

import (
	"fmt"
	"os"

	"github.com/charmbracelet/log"
	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/storage/memory"
	clabconstants "github.com/srl-labs/containerlab/constants"
	clabutils "github.com/srl-labs/containerlab/utils"
)

type GoGit struct {
	gitRepo GitRepo
	r       *gogit.Repository
}

// make sure GoGit satisfies the Git interface.
var _ Git = (*GoGit)(nil)

func NewGoGit(gitRepo GitRepo) *GoGit {
	return &GoGit{
		gitRepo: gitRepo,
	}
}

// Clone takes the given GitRepo reference and clones the repo
// with its internal implementation.
func (g *GoGit) Clone() error {
	// if the directory is not present
	if s, err := os.Stat(g.gitRepo.GetName()); os.IsNotExist(err) {
		return g.cloneNonExisting()
	} else if s.IsDir() {
		return g.cloneExistingRepo()
	}
	return fmt.Errorf("error %q exists already but is a file", g.gitRepo.GetName())
}

func (g *GoGit) getDefaultBranch() (string, error) {
	rem := gogit.NewRemote(memory.NewStorage(), &config.RemoteConfig{
		Name: "origin",
		URLs: []string{g.gitRepo.GetCloneURL().String()},
	})

	// We can then use every Remote functions to retrieve wanted information
	refs, err := rem.List(&gogit.ListOptions{})
	if err != nil {
		return "", err
	}

	for _, ref := range refs {
		if ref.Type() == plumbing.SymbolicReference && ref.Name() == plumbing.HEAD {
			return ref.Target().Short(), nil
		}
	}

	return "", fmt.Errorf("unable to determine default branch for %q", g.gitRepo.GetCloneURL().String())
}

func (g *GoGit) openRepo() error {
	var err error

	// load the git repository
	g.r, err = gogit.PlainOpen(g.gitRepo.GetName())
	if err != nil {
		return err
	}
	return nil
}

func (g *GoGit) cloneExistingRepo() error {
	log.Debugf("loading git repository %q", g.gitRepo.GetName())

	// open the existing repo
	err := g.openRepo()
	if err != nil {
		return err
	}

	// loading remote
	remote, err := g.r.Remote("origin")
	if err != nil {
		return err
	}

	// checking that the configured remote equals the provided remote
	if remote.Config().URLs[0] != g.gitRepo.GetCloneURL().String() {
		return fmt.Errorf("repository url of %q differs (%q) from the provided url (%q). stopping",
			g.gitRepo.GetName(), remote.Config().URLs[0], g.gitRepo.GetCloneURL().String())
	}

	// get the worktree reference
	tree, err := g.r.Worktree()
	if err != nil {
		return err
	}

	// resolve the branch
	// the branch ref from the URL might be empty -> ""
	// then we need to figure out whats the default branch main / master / sth. else.
	branch := g.gitRepo.GetBranch()
	if branch == "" {
		log.Debugf("default branch not set. determining it")
		branch, err = g.getDefaultBranch()
		if err != nil {
			return err
		}
		log.Debugf("default branch is %q", branch)
	}

	// prepare the checkout options
	checkoutOpts := &gogit.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName(branch),
	}

	// check if the branch already exists locally.
	// if not fetch it and check it out.

	if _, err = g.r.Reference(plumbing.NewBranchReferenceName(branch), false); err != nil {
		err = g.fetchNonExistingBranch(branch)
		if err != nil {
			return err
		}

		ref, err := g.r.Reference(plumbing.NewRemoteReferenceName("origin", branch), true)
		if err != nil {
			return err
		}

		checkoutOpts.Hash = ref.Hash()
		checkoutOpts.Create = true
	}

	log.Debugf("checking out branch %q", branch)

	// execute the checkout
	err = tree.Checkout(checkoutOpts)
	if err != nil {
		return err
	}

	log.Debug("pulling latest repo data")
	// init the pull options
	pullOpts := &gogit.PullOptions{
		Depth:        1,
		SingleBranch: true,
		Force:        true,
	}
	// execute the pull
	err = tree.Pull(pullOpts)
	if err == gogit.NoErrAlreadyUpToDate {
		log.Debugf("git repository up to date")
		err = nil
	}

	return err
}

func (g *GoGit) fetchNonExistingBranch(branch string) error {
	// init the remote
	remote, err := g.r.Remote("origin")
	if err != nil {
		return err
	}

	// build the RefSpec, that wires the remote to the locla branch
	localRef := plumbing.NewBranchReferenceName(branch)
	remoteRef := plumbing.NewRemoteReferenceName("origin", branch)
	refSpec := config.RefSpec(fmt.Sprintf("+%s:%s", localRef, remoteRef))

	// init fetch options
	fetchOpts := &gogit.FetchOptions{
		Depth:    1,
		RefSpecs: []config.RefSpec{refSpec},
	}

	// execute the fetch
	err = remote.Fetch(fetchOpts)
	if err == gogit.NoErrAlreadyUpToDate {
		log.Debugf("git repository up to date")
	} else if err != nil {
		return err
	}

	// make sure the branch is also showing up in .git/config
	err = g.r.CreateBranch(&config.Branch{
		Name:   branch,
		Remote: "origin",
		Merge:  localRef,
	})

	return err
}

func (g *GoGit) cloneNonExisting() error {
	var err error
	// init clone options
	co := &gogit.CloneOptions{
		Depth:        1,
		URL:          g.gitRepo.GetCloneURL().String(),
		SingleBranch: true,
	}
	// set brach reference if set
	if g.gitRepo.GetBranch() != "" {
		co.ReferenceName = plumbing.NewBranchReferenceName(g.gitRepo.GetBranch())
	} else {
		branchName, err := g.getDefaultBranch()
		if err != nil {
			return err
		}
		co.ReferenceName = plumbing.NewBranchReferenceName(branchName)
	}
	// pre-create the repo directory and adjust the ACLs
	clabutils.CreateDirectory(g.gitRepo.GetName(), clabconstants.PermissionsDirDefault)
	err = clabutils.AdjustFileACLs(g.gitRepo.GetName())
	if err != nil {
		log.Warnf("failed to adjust repository (%s) ACLs. continuin anyways", g.gitRepo.GetName())
	}

	// perform clone
	g.r, err = gogit.PlainClone(g.gitRepo.GetName(), false, co)

	return err
}

type Git interface {
	// Clone takes the given GitRepo reference and clones the repo
	// with its internal implementation.
	Clone() error
}
