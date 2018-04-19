package flagparser

import (
	"fmt"

	"github.com/docker/swarmkit/api"
	"github.com/docker/swarmkit/cmd/swarmctl/common"
	"github.com/docker/swarmkit/cmd/swarmctl/network"
	"github.com/spf13/cobra"
)

func parseMode(cmd *cobra.Command, spec *api.ServiceSpec, c api.ControlClient) error {
	flags := cmd.Flags()

	if flags.Changed("mode") {
		mode, err := flags.GetString("mode")
		if err != nil {
			return err
		}

		switch mode {
		case "global":
			if spec.GetGlobal() == nil {
				spec.Mode = &api.ServiceSpec_Global{
					Global: &api.GlobalService{},
				}
			}
		case "replicated":
			if spec.GetReplicated() == nil {
				spec.Mode = &api.ServiceSpec_Replicated{
					Replicated: &api.ReplicatedService{},
				}
			}
		case "static":
			if spec.GetStatic() == nil {
				spec.Mode = &api.ServiceSpec_Static{
					Static: &api.StaticService{
						Placement: &api.Placement{},
					},
				}
			}
		}
	}

	if flags.Changed("replicas") {
		if spec.GetReplicated() == nil {
			return fmt.Errorf("--replicas can only be specified in --mode replicated")
		}
		replicas, err := flags.GetUint64("replicas")
		if err != nil {
			return err
		}
		spec.GetReplicated().Replicas = replicas
	}

	if flags.Changed("peer-group") {
		if spec.GetStatic() == nil {
			return fmt.Errorf("--peer-group can only be specified in --mode static")
		}
		peerGroup, err := flags.GetString("peer-group")
		if err != nil {
			return err
		}
		spec.GetStatic().PeerGroup = peerGroup
	}

	if flags.Changed("peer-network") {
		if spec.GetStatic() == nil {
			return fmt.Errorf("--peer-network can only be specified in --mode static")
		}
		input, err := flags.GetString("peer-network")
		if err != nil {
			return err
		}
		peerNetwork, err := network.GetNetwork(common.Context(cmd), c, input)
		if err != nil {
			return err
		}
		spec.GetStatic().PeerNetwork = peerNetwork.ID

		// Add this network to the task spec if not already.
		addNetworkAttachmentConfig(&spec.Task, peerNetwork.ID)
	}

	return nil
}
