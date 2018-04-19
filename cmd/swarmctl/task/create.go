package task

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
		Short: "Create a task",
		RunE: func(cmd *cobra.Command, args []string) error {
			if !cmd.Flags().Changed("name") || !cmd.Flags().Changed("image") {
				return errors.New("--name and --image are mandatory")
			}

			followLogs, err := cmd.Flags().GetBool("follow")
			if err != nil {
				return err
			}

			annotations := &api.Annotations{}
			if err := flagparser.MergeAnnotations(cmd, annotations); err != nil {
				return err
			}

			c, err := common.Dial(cmd)
			if err != nil {
				return err
			}

			spec := &api.TaskSpec{
				Runtime: &api.TaskSpec_Container{
					Container: &api.ContainerSpec{},
				},
			}
			if err := flagparser.MergeTask(cmd, spec, c); err != nil {
				return err
			}

			containerSpec := spec.GetContainer()
			containerSpec.Args = append(containerSpec.Args, args...)

			if err := flagparser.ParseAddSecret(cmd, spec, "secret"); err != nil {
				return err
			}

			if err := flagparser.ParseAddConfig(cmd, spec, "config"); err != nil {
				return err
			}

			r, err := c.CreateTask(common.Context(cmd), &api.CreateTaskRequest{Spec: spec, Annotations: annotations})
			if err != nil {
				return err
			}

			if !followLogs {
				fmt.Println(r.Task.ID)
				return nil
			}

			return streamLogs(cmd, true, r.Task.ID)
		},
	}
)

func init() {
	flags := createCmd.Flags()
	flagparser.AddTaskFlags(flags)
	flagparser.AddAnnotationsFlags(flags)
	flags.StringSlice("secret", nil, "add a secret from swarm")
	flags.StringSlice("config", nil, "add a config from swarm")
	flags.BoolP("follow", "f", false, "Follow log output")
}
