package srl

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/srl-labs/containerlab/clab/exec"
)

func (n *srl) setCustomPrompt(tplData *srlTemplateData) {
	// when CLAB_CUSTOM_PROMPT is set to false, we don't generate custom prompt
	if strings.ToLower(os.Getenv("CLAB_CUSTOM_PROMPT")) == "false" {
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
	cmd, _ := exec.NewExecCmdFromString(`sr_cli -d "environment show | grep -A 2 prompt"`)

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
	re, _ := regexp.Compile(`value\s+=\s+"(.+)"`)
	v := re.FindStringSubmatch(s)

	if len(v) != 2 {
		return "", fmt.Errorf("failed to parse prompt from string: %s", s)
	}

	return v[1], nil
}
