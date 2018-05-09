package ca

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
		Short: "Create a certificate authority",
		RunE: func(cmd *cobra.Command, args []string) error {
			if !cmd.Flags().Changed("name") {
				return errors.New("--name is mandatory")
			}

			annotations := &api.Annotations{}
			if err := flagparser.MergeAnnotations(cmd, annotations); err != nil {
				return err
			}

			c, err := common.Dial(cmd)
			if err != nil {
				return err
			}

			spec := &api.CertificateAuthoritySpec{
				Annotations: *annotations,
			}

			r, err := c.CreateCA(common.Context(cmd), &api.CreateCARequest{Spec: spec})
			if err != nil {
				return err
			}

			fmt.Println(r.CertificateAuthority.ID)
			return nil
		},
	}
)

func init() {
	flags := createCmd.Flags()
	flagparser.AddAnnotationsFlags(flags)
}
