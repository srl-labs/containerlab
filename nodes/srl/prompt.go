package srl

import (
	"context"
	"fmt"
	"regexp"

	"github.com/charmbracelet/log"
	clabexec "github.com/srl-labs/containerlab/exec"
)

func (n *srl) setCustomPrompt(tplData *srlTemplateData) {
	if !n.kindSpecificCfg().CustomPrompt {
		return
	}

	tplData.EnableCustomPrompt = true

	// get the current prompt
	prompt, err := n.currentPrompt(context.Background())
	if err != nil {
		log.Errorf("failed to get current prompt: %v", err)

		tplData.EnableCustomPrompt = false

		return
	}

	// adding newline to the prompt for better visual separation
	tplData.CustomPrompt = "\\n" + prompt
}

// currentPrompt returns the current prompt extracted from the environment.
func (n *srl) currentPrompt(ctx context.Context) (string, error) {
	cmd, _ := clabexec.NewExecCmdFromString(`sr_cli -d "environment show | grep -A 2 prompt"`)

	execResult, err := n.RunExec(ctx, cmd)
	if err != nil {
		return "", err
	}

	log.Debugf("fetching prompt for node %s. stdout: %s, stderr: %s", n.Cfg.ShortName,
		execResult.GetStdOutString(), execResult.GetStdErrString())

	return getPrompt(execResult.GetStdOutString())
}

// getPrompt returns the prompt value from a string blob containing the prompt.
// The s is the output of the "environment show | grep -A 2 prompt" command.
func getPrompt(s string) (string, error) {
	re := regexp.MustCompile(`value\s+=\s+"(.+)"`)
	v := re.FindStringSubmatch(s)

	const promptMatchGroups = 2
	if len(v) != promptMatchGroups {
		return "", fmt.Errorf("failed to parse prompt from string: %s", s)
	}

	return v[1], nil
}
