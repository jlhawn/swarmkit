package flagparser

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/docker/swarmkit/api"
	"github.com/docker/swarmkit/cmd/swarmctl/common"
	"github.com/spf13/cobra"
)

// expects cert issuances in the format CA_NAME
func parseCertIssuanceString(certIssuanceString string) (caName, dirName string, err error) {
	tokens := strings.Split(certIssuanceString, ":")

	caName = strings.TrimSpace(tokens[0])

	if caName == "" {
		err = fmt.Errorf("invalid CA name provided")
		return
	}

	if len(tokens) > 1 {
		dirName = strings.TrimSpace(tokens[1])
		if dirName == "" {
			err = fmt.Errorf("invalid directory name provided")
			return
		}
	} else {
		dirName = filepath.Join("/run/certs/", caName)
	}
	return
}

// ParseAddCertIssuance validates cert issuances passed on the command line
func ParseAddCertIssuance(cmd *cobra.Command, spec *api.TaskSpec, flagName string) error {
	flags := cmd.Flags()

	if !flags.Changed(flagName) {
		return nil
	}

	argSpecs, err := flags.GetStringSlice(flagName)
	if err != nil {
		return err
	}

	container := spec.GetContainer()
	if container == nil {
		spec.Runtime = &api.TaskSpec_Container{
			Container: &api.ContainerSpec{},
		}
	}

	lookupCANames := []string{}
	var needCAs []*api.CertificateIssuance

	for _, argSpec := range argSpecs {
		caName, dirName, err := parseCertIssuanceString(argSpec)
		if err != nil {
			return err
		}

		certIssuances := &api.CertificateIssuance{
			CertificateAuthorityName: caName,
			Directory: &api.FileTarget{
				Name: dirName,
				Mode: 0444,
				UID:  "0",
				GID:  "0",
			},
		}

		lookupCANames = append(lookupCANames, caName)
		needCAs = append(needCAs, certIssuances)
	}

	client, err := common.Dial(cmd)
	if err != nil {
		return err
	}

	r, err := client.ListCAs(common.Context(cmd),
		&api.ListCAsRequest{Filters: &api.ListCAsRequest_Filters{Names: lookupCANames}})
	if err != nil {
		return err
	}

	foundCAs := make(map[string]*api.CertificateAuthority)
	for _, ca := range r.CertificateAuthorities {
		foundCAs[ca.Spec.Annotations.Name] = ca
	}

	for _, certIssuance := range needCAs {
		certAuthority, ok := foundCAs[certIssuance.CertificateAuthorityName]
		if !ok {
			return fmt.Errorf("certificate authority not found: %s", certIssuance.CertificateAuthorityName)
		}

		certIssuance.CertificateAuthorityID = certAuthority.ID
		container.CertificateIssuances = append(container.CertificateIssuances, certIssuance)
	}

	return nil
}

// ParseRemoveCertIssuance removes a set of cert issuances from the task spec's
// certificate issuances.
func ParseRemoveCertIssuance(cmd *cobra.Command, spec *api.TaskSpec, flagName string) error {
	flags := cmd.Flags()

	if !flags.Changed(flagName) {
		return nil
	}

	container := spec.GetContainer()
	if container == nil {
		return nil
	}

	caNames, err := flags.GetStringSlice(flagName)
	if err != nil {
		return err
	}

	wantToDelete := make(map[string]struct{})
	for _, caName := range caNames {
		wantToDelete[caName] = struct{}{}
	}

	certIssuances := []*api.CertificateIssuance{}

	for _, certIssuance := range container.CertificateIssuances {
		if _, ok := wantToDelete[certIssuance.CertificateAuthorityName]; ok {
			continue
		}
		certIssuances = append(certIssuances, certIssuance)
	}

	container.CertificateIssuances = certIssuances

	return nil
}
