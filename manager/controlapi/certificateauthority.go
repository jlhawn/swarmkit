package controlapi

import (
	"fmt"

	cfcsr "github.com/cloudflare/cfssl/csr"
	"github.com/cloudflare/cfssl/initca"
	"github.com/docker/swarmkit/api"
	"github.com/docker/swarmkit/ca"
	"github.com/docker/swarmkit/identity"
	"github.com/docker/swarmkit/manager/state/store"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// CreateCA creates and returns a new certificate authority based on the
// provided CertificateAuthoritySpec.
// - Returns `InvalidArgument` if the spec is malformed.
// - Returns `AlreadyExists` if the spec conflicts.
// - Returns an error if the creation fails.
func (s *Server) CreateCA(ctx context.Context, request *api.CreateCARequest) (*api.CreateCAResponse, error) {
	if request.Spec == nil {
		return nil, status.Errorf(codes.InvalidArgument, errInvalidArgument.Error())
	}

	if err := validateAnnotations(request.Spec.Annotations); err != nil {
		return nil, err
	}

	// Generate root key and self-signed certificate.
	rootCN := fmt.Sprintf("%s Root Certificate Authority", request.Spec.Annotations.Name)
	req := cfcsr.CertificateRequest{
		CN:         rootCN,
		KeyRequest: cfcsr.NewBasicKeyRequest(),
		CA:         &cfcsr.CAConfig{Expiry: ca.RootCAExpiration},
	}

	// Generate the CA and get the certificate and private key
	cert, _, key, err := initca.New(&req)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "unable to generate root CA certificate and key: %s", err)
	}

	certAuthority := &api.CertificateAuthority{
		ID:   identity.NewID(),
		Spec: *request.Spec,
		Cert: cert,
		Key:  key,
	}

	err = s.store.Update(func(tx store.Tx) error {
		return store.CreateCA(tx, certAuthority)
	})
	if err != nil {
		return nil, err
	}

	return &api.CreateCAResponse{
		CertificateAuthority: certAuthority,
	}, nil
}

// GetCA returns a CertificateAuthority given an ID.
// - Returns `InvalidArgument` if ID is not provided.
// - Returns `NotFound` if the CA is not found.
func (s *Server) GetCA(ctx context.Context, request *api.GetCARequest) (*api.GetCAResponse, error) {
	if request.CertificateAuthorityID == "" {
		return nil, status.Errorf(codes.InvalidArgument, errInvalidArgument.Error())
	}

	var certAuthority *api.CertificateAuthority
	s.store.View(func(tx store.ReadTx) {
		certAuthority = store.GetCA(tx, request.CertificateAuthorityID)
	})
	if certAuthority == nil {
		return nil, status.Errorf(codes.NotFound, "certificate authority %s not found", request.CertificateAuthorityID)
	}

	// Note: omit the private key from the response.
	certAuthority.Key = nil

	return &api.GetCAResponse{
		CertificateAuthority: certAuthority,
	}, nil
}

// RemoveCA removes a CertificateAuthority referenced by ID.
// - Returns `InvalidArgument` if ID is not provided.
// - Returns `NotFound` if the CA is not found.
// - Returns an error if the deletion fails.
func (s *Server) RemoveCA(ctx context.Context, request *api.RemoveCARequest) (*api.RemoveCAResponse, error) {
	if request.CertificateAuthorityID == "" {
		return nil, status.Errorf(codes.InvalidArgument, errInvalidArgument.Error())
	}

	err := s.store.Update(func(tx store.Tx) error {
		return store.DeleteCA(tx, request.CertificateAuthorityID)
	})
	if err != nil {
		if err == store.ErrNotExist {
			return nil, status.Errorf(codes.NotFound, "certificate authority %s not found", request.CertificateAuthorityID)
		}
		return nil, err
	}
	return &api.RemoveCAResponse{}, nil
}

func filterCAs(candidates []*api.CertificateAuthority, filters ...func(*api.CertificateAuthority) bool) []*api.CertificateAuthority {
	result := []*api.CertificateAuthority{}

	for _, c := range candidates {
		match := true
		for _, f := range filters {
			if !f(c) {
				match = false
				break
			}
		}
		if match {
			result = append(result, c)
		}
	}

	return result
}

// ListCAs returns a list of all CAs.
func (s *Server) ListCAs(ctx context.Context, request *api.ListCAsRequest) (*api.ListCAsResponse, error) {
	var (
		certAuthorities []*api.CertificateAuthority
		err             error
	)

	s.store.View(func(tx store.ReadTx) {
		switch {
		case request.Filters != nil && len(request.Filters.Names) > 0:
			certAuthorities, err = store.FindCAs(tx, buildFilters(store.ByName, request.Filters.Names))
		case request.Filters != nil && len(request.Filters.NamePrefixes) > 0:
			certAuthorities, err = store.FindCAs(tx, buildFilters(store.ByNamePrefix, request.Filters.NamePrefixes))
		case request.Filters != nil && len(request.Filters.IDPrefixes) > 0:
			certAuthorities, err = store.FindCAs(tx, buildFilters(store.ByIDPrefix, request.Filters.IDPrefixes))
		default:
			certAuthorities, err = store.FindCAs(tx, store.All)
		}

		if err != nil || request.Filters == nil {
			return
		}

		certAuthorities = filterCAs(certAuthorities,
			func(e *api.CertificateAuthority) bool {
				return filterContains(e.Spec.Annotations.Name, request.Filters.Names)
			},
			func(e *api.CertificateAuthority) bool {
				return filterContainsPrefix(e.Spec.Annotations.Name, request.Filters.NamePrefixes)
			},
			func(e *api.CertificateAuthority) bool {
				return filterContainsPrefix(e.ID, request.Filters.IDPrefixes)
			},
			func(e *api.CertificateAuthority) bool {
				return filterMatchLabels(e.Spec.Annotations.Labels, request.Filters.Labels)
			},
		)
	})

	if err != nil {
		return nil, err
	}

	for _, certAuthority := range certAuthorities {
		// Note: omit the private key from the response.
		certAuthority.Key = nil
	}

	return &api.ListCAsResponse{
		CertificateAuthorities: certAuthorities,
	}, nil
}
