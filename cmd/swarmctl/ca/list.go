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

var (
	listCmd = &cobra.Command{
		Use:   "ls",
		Short: "List certificate authorities",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 0 {
				return errors.New("ls command takes no arguments")
			}

			flags := cmd.Flags()

			quiet, err := flags.GetBool("quiet")
			if err != nil {
				return err
			}

			c, err := common.Dial(cmd)
			if err != nil {
				return err
			}
			resp, err := c.ListCAs(common.Context(cmd), &api.ListCAsRequest{})
			if err != nil {
				return err
			}

			var output func(t *api.CertificateAuthority)

			if !quiet {
				w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
				defer w.Flush()

				common.PrintHeader(w, "ID", "Name")
				output = func(certAuthority *api.CertificateAuthority) {

					fmt.Fprintf(w, "%s\t%s\t\n",
						certAuthority.ID,
						certAuthority.Spec.Annotations.Name,
					)
				}
			} else {
				output = func(certAuthority *api.CertificateAuthority) { fmt.Println(certAuthority.ID) }
			}

			for _, certAuthority := range resp.CertificateAuthorities {
				output(certAuthority)
			}
			return nil
		},
	}
)

func init() {
	listCmd.Flags().BoolP("quiet", "q", false, "Only display IDs")
}
