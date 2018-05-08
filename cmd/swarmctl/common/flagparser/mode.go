package flagparser

import (
	"fmt"

	"github.com/docker/swarmkit/api"
	"github.com/docker/swarmkit/cmd/swarmctl/common"
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
		peerGroupNameOrID, err := flags.GetString("peer-group")
		if err != nil {
			return err
		}

		resolver := common.NewResolver(cmd, c)

		peerGroup, err := resolver.LookupPeerGroup(peerGroupNameOrID)
		if err != nil {
			return err
		}

		spec.GetStatic().PeerGroup = peerGroup.ID
	}

	return nil
}
