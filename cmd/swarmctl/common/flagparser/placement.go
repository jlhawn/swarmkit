package flagparser

import (
	"github.com/docker/swarmkit/api"
	"github.com/spf13/pflag"
)

func parsePlacement(flags *pflag.FlagSet, spec *api.TaskSpec) error {
	if flags.Changed("constraint") {
		constraints, err := flags.GetStringSlice("constraint")
		if err != nil {
			return err
		}
		if spec.Placement == nil {
			spec.Placement = &api.Placement{}
		}
		spec.Placement.Constraints = constraints
	}

	return nil
}
