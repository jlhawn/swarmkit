package flagparser

import (
	"fmt"
	"math/big"

	"github.com/docker/go-units"
	"github.com/docker/swarmkit/api"
	"github.com/docker/swarmkit/api/genericresource"
	"github.com/spf13/pflag"
)

func parseResourceCPU(flags *pflag.FlagSet, resources *api.Resources, name string) error {
	cpu, err := flags.GetString(name)
	if err != nil {
		return err
	}

	nanoCPUs, ok := new(big.Rat).SetString(cpu)
	if !ok {
		return fmt.Errorf("invalid cpu: %s", cpu)
	}
	cpuRat := new(big.Rat).Mul(nanoCPUs, big.NewRat(1e9, 1))
	if !cpuRat.IsInt() {
		return fmt.Errorf("CPU value cannot have more than 9 decimal places: %s", cpu)
	}
	resources.NanoCPUs = cpuRat.Num().Int64()
	return nil
}

func parseResourceMemory(flags *pflag.FlagSet, resources *api.Resources, name string) error {
	memory, err := flags.GetString(name)
	if err != nil {
		return err
	}

	bytes, err := units.RAMInBytes(memory)
	if err != nil {
		return err
	}

	resources.MemoryBytes = bytes
	return nil
}

func parseResource(flags *pflag.FlagSet, spec *api.TaskSpec) error {
	if flags.Changed("memory-reservation") {
		if spec.Resources == nil {
			spec.Resources = &api.ResourceRequirements{}
		}
		if spec.Resources.Reservations == nil {
			spec.Resources.Reservations = &api.Resources{}
		}
		if err := parseResourceMemory(flags, spec.Resources.Reservations, "memory-reservation"); err != nil {
			return err
		}
	}

	if flags.Changed("memory-limit") {
		if spec.Resources == nil {
			spec.Resources = &api.ResourceRequirements{}
		}
		if spec.Resources.Limits == nil {
			spec.Resources.Limits = &api.Resources{}
		}
		if err := parseResourceMemory(flags, spec.Resources.Limits, "memory-limit"); err != nil {
			return err
		}
	}

	if flags.Changed("cpu-reservation") {
		if spec.Resources == nil {
			spec.Resources = &api.ResourceRequirements{}
		}
		if spec.Resources.Reservations == nil {
			spec.Resources.Reservations = &api.Resources{}
		}
		if err := parseResourceCPU(flags, spec.Resources.Reservations, "cpu-reservation"); err != nil {
			return err
		}
	}

	if flags.Changed("cpu-limit") {
		if spec.Resources == nil {
			spec.Resources = &api.ResourceRequirements{}
		}
		if spec.Resources.Limits == nil {
			spec.Resources.Limits = &api.Resources{}
		}
		if err := parseResourceCPU(flags, spec.Resources.Limits, "cpu-limit"); err != nil {
			return err
		}
	}

	if flags.Changed("generic-resources") {
		if spec.Resources == nil {
			spec.Resources = &api.ResourceRequirements{}
		}
		if spec.Resources.Reservations == nil {
			spec.Resources.Reservations = &api.Resources{}
		}

		cmd, err := flags.GetString("generic-resources")
		if err != nil {
			return err
		}
		spec.Resources.Reservations.Generic, err = genericresource.ParseCmd(cmd)
		if err != nil {
			return err
		}
		err = genericresource.ValidateTask(spec.Resources.Reservations)
		if err != nil {
			return err
		}
	}

	return nil
}
