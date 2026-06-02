// Copyright 2025
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package host_test

import (
	"context"
	"testing"

	clabexec "github.com/srl-labs/containerlab/exec"
	clabnodeshost "github.com/srl-labs/containerlab/nodes/host"
	"github.com/stretchr/testify/assert"
)

func TestRunExec(t *testing.T) {
	// Run a command that does succeed
	out, err := clabnodeshost.RunExec(context.TODO(),
		clabexec.NewExecCmdFromSlice([]string{"true"}))
	assert.NoError(t, err, "Exec should not have failed")
	if assert.NotNil(t, out, "The exec result should not be nil") {
		assert.EqualValues(t, 0, out.ReturnCode, "The return code should be 0")
	}

	// Run a command that does not succeed
	out, err = clabnodeshost.RunExec(context.TODO(),
		clabexec.NewExecCmdFromSlice([]string{"false"}))
	assert.NoError(t, err, "Exec should not have failed")
	if assert.NotNil(t, out, "The exec result should not be nil") {
		assert.EqualValues(t, 1, out.ReturnCode, "The return code should be 0")
	}

	// Run a command that does not exist
	out, err = clabnodeshost.RunExec(context.TODO(),
		clabexec.NewExecCmdFromSlice([]string{"unknown-command-foobar"}))
	assert.Error(t, err, "Exec should have failed")
	assert.Nil(t, out, "The exec result should be nil")
}
