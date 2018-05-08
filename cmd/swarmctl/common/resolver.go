package common

import (
	"fmt"

	"github.com/docker/swarmkit/api"
	"github.com/spf13/cobra"
	"golang.org/x/net/context"
)

// Resolver provides ID to Name resolution.
type Resolver struct {
	cmd   *cobra.Command
	c     api.ControlClient
	ctx   context.Context
	cache map[string]string
}

// NewResolver creates a new Resolver.
func NewResolver(cmd *cobra.Command, c api.ControlClient) *Resolver {
	return &Resolver{
		cmd:   cmd,
		c:     c,
		ctx:   Context(cmd),
		cache: make(map[string]string),
	}
}

func (r *Resolver) get(t interface{}, id string) string {
	switch t.(type) {
	case api.Node:
		res, err := r.c.GetNode(r.ctx, &api.GetNodeRequest{NodeID: id})
		if err != nil {
			return id
		}
		if res.Node.Spec.Annotations.Name != "" {
			return res.Node.Spec.Annotations.Name
		}
		if res.Node.Description == nil {
			return id
		}
		return res.Node.Description.Hostname
	case api.Service:
		res, err := r.c.GetService(r.ctx, &api.GetServiceRequest{ServiceID: id})
		if err != nil {
			return id
		}
		return res.Service.Spec.Annotations.Name
	case api.Task:
		res, err := r.c.GetTask(r.ctx, &api.GetTaskRequest{TaskID: id})
		if err != nil {
			return id
		}
		if res.Task.IsStandalone {
			return res.Task.Annotations.Name
		}
		svc := r.get(api.Service{}, res.Task.ServiceID)
		return fmt.Sprintf("%s.%d", svc, res.Task.Slot)
	default:
		return id
	}
}

// Resolve will attempt to resolve an ID to a Name by querying the manager.
// Results are stored into a cache.
// If the `-n` flag is used in the command-line, resolution is disabled.
func (r *Resolver) Resolve(t interface{}, id string) string {
	if r.cmd.Flags().Changed("no-resolve") {
		return id
	}
	if name, ok := r.cache[id]; ok {
		return name
	}
	name := r.get(t, id)
	r.cache[id] = name
	return name
}

func (r *Resolver) LookupNetwork(nameOrID string) (*api.Network, error) {
	// GetNetwork to match via full ID.
	getResp, err := r.c.GetNetwork(r.ctx, &api.GetNetworkRequest{NetworkID: nameOrID})
	if err != nil {
		// If any error (including NotFound), ListNetworks to match via full name.
		listResp, err := r.c.ListNetworks(r.ctx,
			&api.ListNetworksRequest{
				Filters: &api.ListNetworksRequest_Filters{
					Names: []string{nameOrID},
				},
			},
		)
		if err != nil {
			return nil, err
		}

		if len(listResp.Networks) == 0 {
			return nil, fmt.Errorf("network %s not found", nameOrID)
		}

		if l := len(listResp.Networks); l > 1 {
			return nil, fmt.Errorf("network %s is ambiguous (%d matches found)", nameOrID, l)
		}

		return listResp.Networks[0], nil
	}

	return getResp.Network, nil
}

func (r *Resolver) LookupPeerGroup(nameOrID string) (*api.PeerGroup, error) {
	// GetPeerGroup to match via full ID.
	getResp, err := r.c.GetPeerGroup(r.ctx, &api.GetPeerGroupRequest{PeerGroupID: nameOrID})
	if err != nil {
		// If any error (including NotFound), ListPeerGroups to match via full name.
		listResp, err := r.c.ListPeerGroups(r.ctx,
			&api.ListPeerGroupsRequest{
				Filters: &api.ListPeerGroupsRequest_Filters{
					Names: []string{nameOrID},
				},
			},
		)
		if err != nil {
			return nil, err
		}

		if len(listResp.PeerGroups) == 0 {
			return nil, fmt.Errorf("peer group %s not found", nameOrID)
		}

		if l := len(listResp.PeerGroups); l > 1 {
			return nil, fmt.Errorf("peer group %s is ambiguous (%d matches found)", nameOrID, l)
		}

		return listResp.PeerGroups[0], nil
	}

	return getResp.PeerGroup, nil
}

func (r *Resolver) LookupService(nameOrID string) (*api.Service, error) {
	// GetService to match via full ID.
	getResp, err := r.c.GetService(r.ctx, &api.GetServiceRequest{ServiceID: nameOrID})
	if err != nil {
		// If any error (including NotFound), ListServices to match via full name.
		listResp, err := r.c.ListServices(r.ctx,
			&api.ListServicesRequest{
				Filters: &api.ListServicesRequest_Filters{
					Names: []string{nameOrID},
				},
			},
		)
		if err != nil {
			return nil, err
		}

		if len(listResp.Services) == 0 {
			return nil, fmt.Errorf("service %s not found", nameOrID)
		}

		if l := len(listResp.Services); l > 1 {
			return nil, fmt.Errorf("service %s is ambiguous (%d matches found)", nameOrID, l)
		}

		return listResp.Services[0], nil
	}

	return getResp.Service, nil
}

func (r *Resolver) LookupTask(nameOrID string) (*api.Task, error) {
	// GetTask to match via full ID.
	getResp, err := r.c.GetTask(r.ctx, &api.GetTaskRequest{TaskID: nameOrID})
	if err != nil {
		// If any error (including NotFound), ListTasks to match via full name.
		listResp, err := r.c.ListTasks(r.ctx,
			&api.ListTasksRequest{
				Filters: &api.ListTasksRequest_Filters{
					Names: []string{nameOrID},
				},
			},
		)
		if err != nil {
			return nil, err
		}

		if len(listResp.Tasks) == 0 {
			return nil, fmt.Errorf("task %s not found", nameOrID)
		}

		if l := len(listResp.Tasks); l > 1 {
			return nil, fmt.Errorf("task %s is ambiguous (%d matches found)", nameOrID, l)
		}

		return listResp.Tasks[0], nil
	}

	return getResp.Task, nil
}
