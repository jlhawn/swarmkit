package controlapi

import (
	"time"

	"github.com/docker/swarmkit/api"
	"github.com/docker/swarmkit/api/naming"
	"github.com/docker/swarmkit/identity"
	"github.com/docker/swarmkit/manager/orchestrator"
	"github.com/docker/swarmkit/manager/state/store"
	"github.com/docker/swarmkit/protobuf/ptypes"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// CreateTask creates and returns a stand-alone Task based on the provided
// TaskSpec.
// - Returns `InvalidArgument` if the TaskSpec is malformed.
// - Returns `Unimplemented` if the TaskSpec references unimplemented features.
// - Returns `AlreadyExists` if the TaskSpec conflicts.
// - Returns an error if the creation fails.
func (s *Server) CreateTask(ctx context.Context, request *api.CreateTaskRequest) (*api.CreateTaskResponse, error) {
	if request.Spec == nil || request.Annotations == nil {
		return nil, status.Errorf(codes.InvalidArgument, errInvalidArgument.Error())
	}
	if err := validateAnnotations(*request.Annotations); err != nil {
		return nil, err
	}
	if err := validateTaskSpec(*request.Spec); err != nil {
		return nil, err
	}
	if err := s.validateNetworks(request.Spec.Networks); err != nil {
		return nil, err
	}

	// TODO(aluzzardi): Consider using `Name` as a primary key to handle
	// duplicate creations. See #65
	task := &api.Task{
		ID:          identity.NewID(),
		Spec:        *request.Spec,
		Annotations: *request.Annotations,
		Status: api.TaskStatus{
			State:     api.TaskStateNew,
			Timestamp: ptypes.MustTimestampProto(time.Now()),
			Message:   "created",
		},
		DesiredState: api.TaskStateRunning,
		IsStandalone: true,
	}

	err := s.store.Update(func(tx store.Tx) error {
		// Check to see if all the secrets being added exist as objects
		// in our datastore
		err := s.checkSecretExistence(tx, *request.Spec)
		if err != nil {
			return err
		}
		err = s.checkConfigExistence(tx, *request.Spec)
		if err != nil {
			return err
		}

		if request.Spec.LogDriver != nil {
			// use the log driver specific to the task, if we have it.
			task.LogDriver = request.Spec.LogDriver
		} else {
			// pick up the cluster default, if available.
			// lookup the cluster
			clusters, err := store.FindClusters(tx, store.ByName(store.DefaultClusterName))
			if err != nil {
				return err
			}
			if len(clusters) != 1 {
				return status.Errorf(codes.Internal, "could not fetch cluster object")
			}
			cluster := clusters[0]
			task.LogDriver = cluster.Spec.TaskDefaults.LogDriver
		}

		return store.CreateTask(tx, task)
	})
	if err != nil {
		return nil, err
	}

	return &api.CreateTaskResponse{
		Task: task,
	}, nil
}

// GetTask returns a Task given a TaskID.
// - Returns `InvalidArgument` if TaskID is not provided.
// - Returns `NotFound` if the Task is not found.
func (s *Server) GetTask(ctx context.Context, request *api.GetTaskRequest) (*api.GetTaskResponse, error) {
	if request.TaskID == "" {
		return nil, status.Errorf(codes.InvalidArgument, errInvalidArgument.Error())
	}

	var task *api.Task
	s.store.View(func(tx store.ReadTx) {
		task = store.GetTask(tx, request.TaskID)
	})
	if task == nil {
		return nil, status.Errorf(codes.NotFound, "task %s not found", request.TaskID)
	}
	return &api.GetTaskResponse{
		Task: task,
	}, nil
}

// RemoveTask removes a Task referenced by TaskID.
// - Returns `InvalidArgument` if TaskID is not provided.
// - Returns `NotFound` if the Task is not found.
// - Returns an error if the deletion fails.
func (s *Server) RemoveTask(ctx context.Context, request *api.RemoveTaskRequest) (*api.RemoveTaskResponse, error) {
	if request.TaskID == "" {
		return nil, status.Errorf(codes.InvalidArgument, errInvalidArgument.Error())
	}

	err := s.store.Update(func(tx store.Tx) error {
		return store.DeleteTask(tx, request.TaskID)
	})
	if err != nil {
		if err == store.ErrNotExist {
			return nil, status.Errorf(codes.NotFound, "task %s not found", request.TaskID)
		}
		return nil, err
	}
	return &api.RemoveTaskResponse{}, nil
}

func filterTasks(candidates []*api.Task, filters ...func(*api.Task) bool) []*api.Task {
	result := []*api.Task{}

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

// ListTasks returns a list of all tasks.
func (s *Server) ListTasks(ctx context.Context, request *api.ListTasksRequest) (*api.ListTasksResponse, error) {
	var (
		tasks []*api.Task
		err   error
	)

	s.store.View(func(tx store.ReadTx) {
		switch {
		case request.Filters != nil && len(request.Filters.Names) > 0:
			tasks, err = store.FindTasks(tx, buildFilters(store.ByName, request.Filters.Names))
		case request.Filters != nil && len(request.Filters.NamePrefixes) > 0:
			tasks, err = store.FindTasks(tx, buildFilters(store.ByNamePrefix, request.Filters.NamePrefixes))
		case request.Filters != nil && len(request.Filters.IDPrefixes) > 0:
			tasks, err = store.FindTasks(tx, buildFilters(store.ByIDPrefix, request.Filters.IDPrefixes))
		case request.Filters != nil && len(request.Filters.ServiceIDs) > 0:
			tasks, err = store.FindTasks(tx, buildFilters(store.ByServiceID, request.Filters.ServiceIDs))
		case request.Filters != nil && len(request.Filters.Runtimes) > 0:
			tasks, err = store.FindTasks(tx, buildFilters(store.ByRuntime, request.Filters.Runtimes))
		case request.Filters != nil && len(request.Filters.NodeIDs) > 0:
			tasks, err = store.FindTasks(tx, buildFilters(store.ByNodeID, request.Filters.NodeIDs))
		case request.Filters != nil && len(request.Filters.DesiredStates) > 0:
			filters := make([]store.By, 0, len(request.Filters.DesiredStates))
			for _, v := range request.Filters.DesiredStates {
				filters = append(filters, store.ByDesiredState(v))
			}
			tasks, err = store.FindTasks(tx, store.Or(filters...))
		default:
			tasks, err = store.FindTasks(tx, store.All)
		}

		if err != nil || request.Filters == nil {
			return
		}

		tasks = filterTasks(tasks,
			func(e *api.Task) bool {
				return filterContains(naming.Task(e), request.Filters.Names)
			},
			func(e *api.Task) bool {
				return filterContainsPrefix(naming.Task(e), request.Filters.NamePrefixes)
			},
			func(e *api.Task) bool {
				return filterContainsPrefix(e.ID, request.Filters.IDPrefixes)
			},
			func(e *api.Task) bool {
				return filterMatchLabels(e.ServiceAnnotations.Labels, request.Filters.Labels)
			},
			func(e *api.Task) bool {
				return filterContains(e.ServiceID, request.Filters.ServiceIDs)
			},
			func(e *api.Task) bool {
				return filterContains(e.NodeID, request.Filters.NodeIDs)
			},
			func(e *api.Task) bool {
				if len(request.Filters.Runtimes) == 0 {
					return true
				}
				r, err := naming.Runtime(e.Spec)
				if err != nil {
					return false
				}
				return filterContains(r, request.Filters.Runtimes)
			},
			func(e *api.Task) bool {
				if len(request.Filters.DesiredStates) == 0 {
					return true
				}
				for _, c := range request.Filters.DesiredStates {
					if c == e.DesiredState {
						return true
					}
				}
				return false
			},
			func(e *api.Task) bool {
				if !request.Filters.UpToDate {
					return true
				}

				service := store.GetService(tx, e.ServiceID)
				if service == nil {
					return false
				}

				n := store.GetNode(tx, e.NodeID)
				return !orchestrator.IsTaskDirty(service, e, n)
			},
		)
	})

	if err != nil {
		return nil, err
	}

	return &api.ListTasksResponse{
		Tasks: tasks,
	}, nil
}
