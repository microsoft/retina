package status

import (
	"github.com/pkg/errors"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

var Cmd = &cobra.Command{
	Use:   "status",
	Short: "Output Retina status",
	RunE: func(*cobra.Command, []string) error {
		retinaAgentPs, err := retinaAgentProcess()
		if err != nil {
			return errors.Wrap(err, "failed to get Retina Agent process")
		}
		if retinaAgentPs == nil {
			pterm.Error.Println("Retina Agent is not running")
			return nil
		}
		pterm.Success.Println("Retina Agent is running")

		return nil
	},
}
