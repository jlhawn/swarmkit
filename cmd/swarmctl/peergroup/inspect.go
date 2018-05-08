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

func printPeerGroup(peerGroup *api.PeerGroup, resolver *common.Resolver) {
	w := tabwriter.NewWriter(os.Stdout, 8, 8, 8, ' ', 0)
	defer w.Flush()

	networkTxt := peerGroup.Spec.Network
	network, err := resolver.LookupNetwork(networkTxt)
	if err != nil {
		networkTxt = network.Spec.Annotations.Name
	}

	fmt.Fprintf(w, "ID\t: %s\n", peerGroup.ID)
	fmt.Fprintf(w, "Name\t: %s\n", peerGroup.Spec.Annotations.Name)
	fmt.Fprintf(w, "Network\t: %s\n", networkTxt)
	if peerGroup.Endpoint != nil {
		for _, virtualIP := range peerGroup.Endpoint.VirtualIPs {
			fmt.Fprintf(w, "Virtual IP\t: %s\n", virtualIP.Addr)
		}
	}

	fmt.Fprintln(w, "Labels\t")
	for key, value := range peerGroup.Spec.Annotations.Labels {
		fmt.Fprintf(w, "  %s\t: %s\n", key, value)
	}
}

var (
	inspectCmd = &cobra.Command{
		Use:   "inspect <peer group ID>",
		Short: "Inspect a peer group",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return errors.New("peer group ID missing")
			}

			if len(args) > 1 {
				return errors.New("inspect command takes exactly 1 argument")
			}

			c, err := common.Dial(cmd)
			if err != nil {
				return err
			}

			resolver := common.NewResolver(cmd, c)

			peerGroup, err := resolver.LookupPeerGroup(args[0])
			if err != nil {
				return err
			}

			printPeerGroup(peerGroup, resolver)

			return nil
		},
	}
)
