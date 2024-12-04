package main

import (
	"bytes"
	"testing"

	"github.com/microsoft/retina/cli/cmd"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func ExecuteInTest(cmd *cobra.Command, args []string) (stdout string, stderr string, err error) {
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.SetArgs(args)
	cmd.SetOut(&stdoutBuf)
	cmd.SetErr(&stderrBuf)
	defer func() {
		cmd.SetArgs(nil)
		cmd.SetOut(nil)
		cmd.SetErr(nil)
	}()

	err = cmd.Execute()
	return stdoutBuf.String(), stderrBuf.String(), err
}

func TestCLICmd(t *testing.T) {
	stdout, stderr, err := ExecuteInTest(cmd.Retina, []string{"-h"})
	require.NoError(t, err)
	assert.Contains(t, stdout, "kubectl-retina")
	assert.Equal(t, stderr, "")
}
