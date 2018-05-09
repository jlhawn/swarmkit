package ca

import (
	"errors"
	"fmt"

	"github.com/docker/swarmkit/api"
	"github.com/docker/swarmkit/cmd/swarmctl/common"
	"github.com/spf13/cobra"
)

var (
	removeCmd = &cobra.Command{
		Use:     "remove <CA ID>",
		Short:   "Remove a certificate authority",
		Aliases: []string{"rm"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return errors.New("certificate authority ID missing")
			}

			if len(args) > 1 {
				return errors.New("remove command takes exactly 1 argument")
			}

			c, err := common.Dial(cmd)
			if err != nil {
				return err
			}

			resolver := common.NewResolver(cmd, c)
			certAuthority, err := resolver.LookupCA(args[0])
			if err != nil {
				return err
			}

			_, err = c.RemoveCA(common.Context(cmd), &api.RemoveCARequest{CertificateAuthorityID: certAuthority.ID})
			if err != nil {
				return err
			}
			fmt.Println(args[0])
			return nil
		},
	}
)
