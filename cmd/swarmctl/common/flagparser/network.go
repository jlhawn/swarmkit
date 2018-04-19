package flagparser

import (
	"github.com/docker/swarmkit/api"
	"github.com/docker/swarmkit/cmd/swarmctl/common"
	"github.com/docker/swarmkit/cmd/swarmctl/network"
	"github.com/spf13/cobra"
)

func parseNetworks(cmd *cobra.Command, spec *api.TaskSpec, c api.ControlClient) error {
	flags := cmd.Flags()
	if !flags.Changed("network") {
		return nil
	}
	networkInputs, err := flags.GetStringSlice("network")
	if err != nil {
		return err
	}

	for _, input := range networkInputs {
		n, err := network.GetNetwork(common.Context(cmd), c, input)
		if err != nil {
			return err
		}

		addNetworkAttachmentConfig(spec, n.ID)
	}

	return nil
}

// addNetworkAttachmentConfig adds the given networkID as a network attachment
// to the given task spec if it is not already.
func addNetworkAttachmentConfig(spec *api.TaskSpec, networkID string) {
	for _, attachmentConfig := range spec.Networks {
		if attachmentConfig.Target == networkID {
			return // Spec already has the peer network.
		}
	}

	spec.Networks = append(spec.Networks, &api.NetworkAttachmentConfig{
		Target: networkID,
	})
}
