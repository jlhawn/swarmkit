package service

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
		Short: "Create a service",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 0 {
				return errors.New("create command takes no arguments")
			}

			if !cmd.Flags().Changed("name") || !cmd.Flags().Changed("image") {
				return errors.New("--name and --image are mandatory")
			}

			c, err := common.Dial(cmd)
			if err != nil {
				return err
			}

			spec := &api.ServiceSpec{
				Mode: &api.ServiceSpec_Replicated{
					Replicated: &api.ReplicatedService{
						Replicas: 1,
					},
				},
				Task: api.TaskSpec{
					Runtime: &api.TaskSpec_Container{
						Container: &api.ContainerSpec{},
					},
				},
			}

			if err := flagparser.MergeService(cmd, spec, c); err != nil {
				return err
			}

			if err := flagparser.ParseAddSecret(cmd, &spec.Task, "secret"); err != nil {
				return err
			}
			if err := flagparser.ParseAddConfig(cmd, &spec.Task, "config"); err != nil {
				return err
			}

			// Additional checks for static mode.
			if spec.GetStatic() != nil {
				flags := cmd.Flags()
				if !flags.Changed("peer-group") {
					return fmt.Errorf("--peer-group is required with --mode=static")
				}
				// Transfer the placement config to the mode.
				spec.GetStatic().Placement = spec.Task.Placement
				spec.Task.Placement = nil
			}

			r, err := c.CreateService(common.Context(cmd), &api.CreateServiceRequest{Spec: spec})
			if err != nil {
				return err
			}
			fmt.Println(r.Service.ID)
			return nil
		},
	}
)

func init() {
	flags := createCmd.Flags()
	flagparser.AddServiceFlags(flags)
	flagparser.AddAnnotationsFlags(flags)
	flagparser.AddTaskFlags(flags)
	flags.String("mode", "replicated", "one of replicated, global,static")
	flags.String("peer-group", "", "name of peer group if static mode")
	flags.StringSlice("secret", nil, "add a secret from swarm")
	flags.StringSlice("config", nil, "add a config from swarm")
}
