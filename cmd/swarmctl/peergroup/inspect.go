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

func printPeerGroup(peerGroup *api.PeerGroup, resolver *common.Resolver, services []*api.Service) {
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

	fmt.Fprintln(w, "\nServices:\n")
	common.PrintHeader(w, "ID", "Name", "Image")
	for _, service := range services {
		spec := service.Spec

		var reference string
		if spec.Task.GetContainer() != nil {
			reference = spec.Task.GetContainer().Image
		}

		fmt.Fprintf(w, "%s\t%s\t%s\n",
			service.ID,
			spec.Annotations.Name,
			reference,
		)
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

			resp, err := c.ListServices(common.Context(cmd), &api.ListServicesRequest{
				Filters: &api.ListServicesRequest_Filters{
					PeerGroups: []string{peerGroup.ID},
				},
			})
			if err != nil {
				return err
			}

			printPeerGroup(peerGroup, resolver, resp.Services)

			return nil
		},
	}
)
