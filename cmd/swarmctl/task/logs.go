package task

import (
	"fmt"
	"io"
	"os"

	"github.com/docker/swarmkit/api"
	"github.com/docker/swarmkit/cmd/swarmctl/common"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"golang.org/x/net/context"
)

var (
	logsCmd = &cobra.Command{
		Use:     "logs <task ID...>",
		Short:   "Obtain log output from a task",
		Aliases: []string{"log"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return errors.New("missing task IDs")
			}

			follow, err := cmd.Flags().GetBool("follow")
			if err != nil {
				return err
			}

			conn, err := common.DialConn(cmd)
			if err != nil {
				return err
			}

			c := api.NewControlClient(conn)

			taskIDs := []string{}
			for _, arg := range args {
				task, err := getTask(common.Context(cmd), c, arg)
				if err != nil {
					return err
				}
				taskIDs = append(taskIDs, task.ID)
			}

			return streamLogs(cmd, follow, taskIDs...)
		},
	}
)

func streamLogs(cmd *cobra.Command, follow bool, taskIDs ...string) error {
	conn, err := common.DialConn(cmd)
	if err != nil {
		return err
	}

	c := api.NewControlClient(conn)
	r := common.NewResolver(cmd, c)

	client := api.NewLogsClient(conn)
	ctx := context.Background()
	stream, err := client.SubscribeLogs(ctx, &api.SubscribeLogsRequest{
		Selector: &api.LogSelector{
			TaskIDs: taskIDs,
		},
		Options: &api.LogSubscriptionOptions{
			Follow: follow,
		},
	})
	if err != nil {
		return errors.Wrap(err, "failed to subscribe to logs")
	}

	for {
		log, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return errors.Wrap(err, "failed receiving stream message")
		}

		for _, msg := range log.Messages {
			out := os.Stdout
			if msg.Stream == api.LogStreamStderr {
				out = os.Stderr
			}

			fmt.Fprintf(out, "%s@%s❯ ",
				r.Resolve(api.Task{}, msg.Context.TaskID),
				r.Resolve(api.Node{}, msg.Context.NodeID),
			)
			out.Write(msg.Data) // assume new line?
		}
	}
}

func getTask(ctx context.Context, c api.ControlClient, input string) (*api.Task, error) {
	// GetTask to match via full ID.
	getResp, err := c.GetTask(ctx, &api.GetTaskRequest{TaskID: input})
	if err == nil {
		return getResp.Task, nil
	}

	// If any error (including NotFound), ListTasks to match via full name.
	listResp, err := c.ListTasks(ctx,
		&api.ListTasksRequest{
			Filters: &api.ListTasksRequest_Filters{
				Names: []string{input},
			},
		},
	)
	if err != nil {
		return nil, err
	}

	if len(listResp.Tasks) == 0 {
		return nil, fmt.Errorf("task not found with name %s", input)
	}

	if l := len(listResp.Tasks); l > 1 {
		return nil, fmt.Errorf("task name %s is ambiguous (%d matches found)", input, l)
	}

	return listResp.Tasks[0], nil

}

func init() {
	logsCmd.Flags().BoolP("follow", "f", false, "Follow log output")
}
