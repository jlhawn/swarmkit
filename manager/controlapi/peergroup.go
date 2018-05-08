package controlapi

import (
	"github.com/docker/swarmkit/api"
	"github.com/docker/swarmkit/identity"
	"github.com/docker/swarmkit/manager/state/store"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// CreatePeerGroup creates and returns a new peer group based on the provided
// PeerGroupSpec.
// - Returns `InvalidArgument` if the spec is malformed.
// - Returns `AlreadyExists` if the spec conflicts.
// - Returns an error if the creation fails.
func (s *Server) CreatePeerGroup(ctx context.Context, request *api.CreatePeerGroupRequest) (*api.CreatePeerGroupResponse, error) {
	if request.Spec == nil {
		return nil, status.Errorf(codes.InvalidArgument, errInvalidArgument.Error())
	}

	if err := validateAnnotations(request.Spec.Annotations); err != nil {
		return nil, err
	}

	networkAttachments := []*api.NetworkAttachmentConfig{
		{
			Target: request.Spec.Network,
		},
	}
	if err := s.validateNetworks(networkAttachments); err != nil {
		return nil, err
	}

	peerGroup := &api.PeerGroup{
		ID:   identity.NewID(),
		Spec: *request.Spec,
	}

	err := s.store.Update(func(tx store.Tx) error {
		return store.CreatePeerGroup(tx, peerGroup)
	})
	if err != nil {
		return nil, err
	}

	return &api.CreatePeerGroupResponse{
		PeerGroup: peerGroup,
	}, nil
}

// GetPeerGroup returns a PeerGroup given an ID.
// - Returns `InvalidArgument` if ID is not provided.
// - Returns `NotFound` if the PeerGroup is not found.
func (s *Server) GetPeerGroup(ctx context.Context, request *api.GetPeerGroupRequest) (*api.GetPeerGroupResponse, error) {
	if request.PeerGroupID == "" {
		return nil, status.Errorf(codes.InvalidArgument, errInvalidArgument.Error())
	}

	var peerGroup *api.PeerGroup
	s.store.View(func(tx store.ReadTx) {
		peerGroup = store.GetPeerGroup(tx, request.PeerGroupID)
	})
	if peerGroup == nil {
		return nil, status.Errorf(codes.NotFound, "peer group %s not found", request.PeerGroupID)
	}
	return &api.GetPeerGroupResponse{
		PeerGroup: peerGroup,
	}, nil
}

// RemovePeerGroup removes a PeerGroup referenced by ID.
// - Returns `InvalidArgument` if ID is not provided.
// - Returns `NotFound` if the PeerGroup is not found.
// - Returns an error if the deletion fails.
func (s *Server) RemovePeerGroup(ctx context.Context, request *api.RemovePeerGroupRequest) (*api.RemovePeerGroupResponse, error) {
	if request.PeerGroupID == "" {
		return nil, status.Errorf(codes.InvalidArgument, errInvalidArgument.Error())
	}

	err := s.store.Update(func(tx store.Tx) error {
		return store.DeletePeerGroup(tx, request.PeerGroupID)
	})
	if err != nil {
		if err == store.ErrNotExist {
			return nil, status.Errorf(codes.NotFound, "peer group %s not found", request.PeerGroupID)
		}
		return nil, err
	}
	return &api.RemovePeerGroupResponse{}, nil
}

func filterPeerGroups(candidates []*api.PeerGroup, filters ...func(*api.PeerGroup) bool) []*api.PeerGroup {
	result := []*api.PeerGroup{}

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

// ListPeerGroups returns a list of all peer groups.
func (s *Server) ListPeerGroups(ctx context.Context, request *api.ListPeerGroupsRequest) (*api.ListPeerGroupsResponse, error) {
	var (
		peerGroups []*api.PeerGroup
		err        error
	)

	s.store.View(func(tx store.ReadTx) {
		switch {
		case request.Filters != nil && len(request.Filters.Names) > 0:
			peerGroups, err = store.FindPeerGroups(tx, buildFilters(store.ByName, request.Filters.Names))
		case request.Filters != nil && len(request.Filters.NamePrefixes) > 0:
			peerGroups, err = store.FindPeerGroups(tx, buildFilters(store.ByNamePrefix, request.Filters.NamePrefixes))
		case request.Filters != nil && len(request.Filters.IDPrefixes) > 0:
			peerGroups, err = store.FindPeerGroups(tx, buildFilters(store.ByIDPrefix, request.Filters.IDPrefixes))
		default:
			peerGroups, err = store.FindPeerGroups(tx, store.All)
		}

		if err != nil || request.Filters == nil {
			return
		}

		peerGroups = filterPeerGroups(peerGroups,
			func(e *api.PeerGroup) bool {
				return filterContains(e.Spec.Annotations.Name, request.Filters.Names)
			},
			func(e *api.PeerGroup) bool {
				return filterContainsPrefix(e.Spec.Annotations.Name, request.Filters.NamePrefixes)
			},
			func(e *api.PeerGroup) bool {
				return filterContainsPrefix(e.ID, request.Filters.IDPrefixes)
			},
			func(e *api.PeerGroup) bool {
				return filterMatchLabels(e.Spec.Annotations.Labels, request.Filters.Labels)
			},
		)
	})

	if err != nil {
		return nil, err
	}

	return &api.ListPeerGroupsResponse{
		PeerGroups: peerGroups,
	}, nil
}
