package peergroup

import "github.com/spf13/cobra"

var (
	// Cmd exposes the top-level task command.
	Cmd = &cobra.Command{
		Use:   "peer-group",
		Short: "Peer Group management",
	}
)

func init() {
	Cmd.AddCommand(
		createCmd,
		listCmd,
		inspectCmd,
		removeCmd,
	)
}
