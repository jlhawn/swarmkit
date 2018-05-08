package peergroup

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
		Short: "List peer groups",
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
			resp, err := c.ListPeerGroups(common.Context(cmd), &api.ListPeerGroupsRequest{})
			if err != nil {
				return err
			}

			var output func(t *api.PeerGroup)

			if !quiet {
				w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
				defer w.Flush()

				common.PrintHeader(w, "ID", "Name", "Network", "Virtual IP")
				output = func(peerGroup *api.PeerGroup) {
					virtualIP := "-"
					if peerGroup.Endpoint != nil && len(peerGroup.Endpoint.VirtualIPs) > 0 {
						virtualIP = peerGroup.Endpoint.VirtualIPs[0].Addr
					}

					fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
						peerGroup.ID,
						peerGroup.Spec.Annotations.Name,
						peerGroup.Spec.Network,
						virtualIP,
					)
				}
			} else {
				output = func(peerGroup *api.PeerGroup) { fmt.Println(peerGroup.ID) }
			}

			for _, peerGroup := range resp.PeerGroups {
				output(peerGroup)
			}
			return nil
		},
	}
)

func init() {
	listCmd.Flags().BoolP("quiet", "q", false, "Only display IDs")
}
