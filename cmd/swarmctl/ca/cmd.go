package ca

import "github.com/spf13/cobra"

var (
	// Cmd exposes the top-level task command.
	Cmd = &cobra.Command{
		Use:   "ca",
		Short: "Certificate Authority management",
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
