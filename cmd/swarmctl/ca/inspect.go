package ca

import (
	"errors"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/docker/swarmkit/api"
	"github.com/docker/swarmkit/cmd/swarmctl/common"
	"github.com/spf13/cobra"
)

func printCA(certAuthority *api.CertificateAuthority) {
	w := tabwriter.NewWriter(os.Stdout, 8, 8, 8, ' ', 0)
	defer w.Flush()

	fmt.Fprintf(w, "ID\t: %s\n", certAuthority.ID)
	fmt.Fprintf(w, "Name\t: %s\n", certAuthority.Spec.Annotations.Name)
	fmt.Fprintln(w, "Labels\t")
	for key, value := range certAuthority.Spec.Annotations.Labels {
		fmt.Fprintf(w, "  %s\t: %s\n", key, value)
	}

	fmt.Fprintf(w, "\n%s\n", string(certAuthority.Cert))
}

var (
	inspectCmd = &cobra.Command{
		Use:   "inspect <CA ID>",
		Short: "Inspect a certificate authority",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return errors.New("certificate authority ID missing")
			}

			if len(args) > 1 {
				return errors.New("inspect command takes exactly 1 argument")
			}

			c, err := common.Dial(cmd)
			if err != nil {
				return err
			}

			resolver := common.NewResolver(cmd, c)
			certAuthority, err := resolver.LookupCA(args[0])
			if err != nil {
				return err
			}

			printCA(certAuthority)

			return nil
		},
	}
)
