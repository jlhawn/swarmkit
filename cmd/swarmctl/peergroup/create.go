package peergroup

import (
	"errors"
	"fmt"

	"github.com/docker/swarmkit/api"
	"github.com/docker/swarmkit/cmd/swarmctl/common"
	"github.com/docker/swarmkit/cmd/swarmctl/common/flagparser"
	"github.com/spf13/cobra"
)

var (
	createCmd = &cobra.Command{
		Use:   "create",
		Short: "Create a peer group",
		RunE: func(cmd *cobra.Command, args []string) error {
			if !cmd.Flags().Changed("name") || !cmd.Flags().Changed("network") {
				return errors.New("--name and --network are mandatory")
			}

			annotations := &api.Annotations{}
			if err := flagparser.MergeAnnotations(cmd, annotations); err != nil {
				return err
			}

			c, err := common.Dial(cmd)
			if err != nil {
				return err
			}

			networkInput, err := cmd.Flags().GetString("network")
			if err != nil {
				return err
			}

			resolver := common.NewResolver(cmd, c)
			peerNetwork, err := resolver.LookupNetwork(networkInput)
			if err != nil {
				return err
			}

			spec := &api.PeerGroupSpec{
				Annotations: *annotations,
				Network:     peerNetwork.ID,
			}

			r, err := c.CreatePeerGroup(common.Context(cmd), &api.CreatePeerGroupRequest{Spec: spec})
			if err != nil {
				return err
			}

			fmt.Println(r.PeerGroup.ID)
			return nil
		},
	}
)

func init() {
	flags := createCmd.Flags()
	flagparser.AddAnnotationsFlags(flags)
	flags.String("network", "", "specify the peer network")
}
