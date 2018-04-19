package flagparser

import (
	"github.com/docker/swarmkit/api"
	gogotypes "github.com/gogo/protobuf/types"
	"github.com/spf13/pflag"
)

// parseContainer parses container options.
func parseContainer(flags *pflag.FlagSet, spec *api.TaskSpec) error {
	if flags.Changed("image") {
		image, err := flags.GetString("image")
		if err != nil {
			return err
		}
		spec.GetContainer().Image = image
	}

	if flags.Changed("hostname") {
		hostname, err := flags.GetString("hostname")
		if err != nil {
			return err
		}
		spec.GetContainer().Hostname = hostname
	}

	if flags.Changed("command") {
		command, err := flags.GetStringSlice("command")
		if err != nil {
			return err
		}
		spec.GetContainer().Command = command
	}

	if flags.Changed("args") {
		args, err := flags.GetStringSlice("args")
		if err != nil {
			return err
		}
		spec.GetContainer().Args = args
	}

	if flags.Changed("env") {
		env, err := flags.GetStringSlice("env")
		if err != nil {
			return err
		}
		spec.GetContainer().Env = env
	}

	if flags.Changed("tty") {
		tty, err := flags.GetBool("tty")
		if err != nil {
			return err
		}

		spec.GetContainer().TTY = tty
	}

	if flags.Changed("open-stdin") {
		openStdin, err := flags.GetBool("open-stdin")
		if err != nil {
			return err
		}

		spec.GetContainer().OpenStdin = openStdin
	}

	if flags.Changed("init") {
		init, err := flags.GetBool("init")
		if err != nil {
			return err
		}

		spec.GetContainer().Init = &gogotypes.BoolValue{
			Value: init,
		}
	}

	return nil
}
