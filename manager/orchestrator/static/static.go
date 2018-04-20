// Package static implements the orchestrator for static services.
//
// Static services are different than replicated services in that there is only
// ever a single replica and it is scheduled permanently to a node and tasks
// are never assigned to a different node.
package static

import (
	"github.com/docker/swarmkit/api"
	"github.com/docker/swarmkit/log"
	"github.com/docker/swarmkit/manager/orchestrator"
	"github.com/docker/swarmkit/manager/orchestrator/restart"
	"github.com/docker/swarmkit/manager/orchestrator/taskinit"
	"github.com/docker/swarmkit/manager/orchestrator/update"
	"github.com/docker/swarmkit/manager/state/store"
	"golang.org/x/net/context"
)

// Orchestrator runs a reconciliation loop to create and destroy tasks as
// necessary for static services.
type Orchestrator struct {
	store *store.MemoryStore
	// servicesByNodeID is a set of service IDs grouped by node ID.
	servicesByNodeID map[string]map[string]struct{}
	// services has all the static services in the cluster, indexed by
	// ServiceID
	services     map[string]*api.Service
	restartTasks map[string]struct{}

	// stopChan signals to the state machine to stop running.
	stopChan chan struct{}
	// doneChan is closed when the state machine terminates.
	doneChan chan struct{}

	updater  *update.Supervisor
	restarts *restart.Supervisor

	cluster *api.Cluster // local instance of the cluster
}

// NewStaticOrchestrator creates a new static Orchestrator
func NewStaticOrchestrator(store *store.MemoryStore) *Orchestrator {
	restartSupervisor := restart.NewSupervisor(store)
	updater := update.NewSupervisor(store, restartSupervisor)
	return &Orchestrator{
		store:            store,
		servicesByNodeID: make(map[string]map[string]struct{}),
		services:         make(map[string]*api.Service),
		stopChan:         make(chan struct{}),
		doneChan:         make(chan struct{}),
		updater:          updater,
		restarts:         restartSupervisor,
		restartTasks:     make(map[string]struct{}),
	}
}

func (o *Orchestrator) initTasks(ctx context.Context, readTx store.ReadTx) error {
	return taskinit.CheckTasks(ctx, o.store, readTx, o, o.restarts)
}

// Run contains the static orchestrator event loop.
func (o *Orchestrator) Run(ctx context.Context) error {
	defer close(o.doneChan)

	// Watch changes to services and tasks
	queue := o.store.WatchQueue()
	watcher, cancel := queue.Watch()
	defer cancel()

	// lookup the cluster
	var err error
	o.store.View(func(readTx store.ReadTx) {
		var clusters []*api.Cluster
		clusters, err = store.FindClusters(readTx, store.ByName(store.DefaultClusterName))

		if len(clusters) != 1 {
			return // just pick up the cluster when it is created.
		}
		o.cluster = clusters[0]
	})
	if err != nil {
		return err
	}

	// Lookup services and add all static services to our set of static
	// services.
	var existingServices []*api.Service
	o.store.View(func(readTx store.ReadTx) {
		existingServices, err = store.FindServices(readTx, store.All)
	})
	if err != nil {
		return err
	}

	var reconcileServiceIDs []string
	for _, s := range existingServices {
		if orchestrator.IsStaticService(s) {
			o.updateService(s)
			reconcileServiceIDs = append(reconcileServiceIDs, s.ID)
		}
	}

	// Fix tasks in store before reconciliation loop.
	o.store.View(func(readTx store.ReadTx) {
		err = o.initTasks(ctx, readTx)
	})
	if err != nil {
		return err
	}

	o.tickTasks(ctx)
	o.reconcileServices(ctx, reconcileServiceIDs)

	for {
		select {
		case event := <-watcher:
			switch v := event.(type) {
			case api.EventUpdateCluster:
				o.cluster = v.Cluster
			case api.EventCreateService:
				if !o.IsRelatedService(v.Service) {
					continue
				}
				// Add to our list of services
				o.updateService(v.Service)
				o.reconcileServices(ctx, []string{v.Service.ID})
			case api.EventUpdateService:
				if !o.IsRelatedService(v.Service) {
					continue
				}
				o.updateService(v.Service)
				o.reconcileServices(ctx, []string{v.Service.ID})
			case api.EventDeleteService:
				if !o.IsRelatedService(v.Service) {
					continue
				}
				orchestrator.SetServiceTasksRemove(ctx, o.store, v.Service)
				// delete the service from service map and node ID index.
				delete(o.services, v.Service.ID)
				delete(o.servicesByNodeID[v.Service.StaticInfo.NodeID], v.Service.ID)
				o.restarts.ClearServiceHistory(v.Service.ID)
			case api.EventDeleteNode:
				o.foreachTaskFromNode(ctx, v.Node, o.deleteTask)

				// Mark static services scheduled to this node as permanently
				// down.
				if err := o.store.Update(func(tx store.Tx) error {
					for serviceID := range o.servicesByNodeID[v.Node.ID] {
						service := store.GetService(tx, serviceID)
						if service == nil {
							continue
						}
						service.StaticInfo.Message = "Static Node Removed"
						if err := store.UpdateService(tx, service); err != nil {
							log.G(ctx).WithError(err).Errorf("unable to update static service %s message after node %s deleted", service.ID, v.Node.ID)
						}
					}
					return nil
				}); err != nil {
					log.G(ctx).WithError(err).Errorf("unable to update static service messages after node %s deleted", v.Node.ID)
				}
			case api.EventUpdateTask:
				o.handleTaskChange(ctx, v.Task)
			}
		case <-o.stopChan:
			return nil
		}
		o.tickTasks(ctx)
	}
}

// FixTask validates a task with the current cluster settings, and takes
// action to make it conformant to node state and service constraint
// it's called at orchestrator initialization
func (o *Orchestrator) FixTask(ctx context.Context, batch *store.Batch, t *api.Task) {
	if _, exists := o.services[t.ServiceID]; !exists {
		return
	}
	// if a task's DesiredState has past running, the task has been processed
	if t.DesiredState > api.TaskStateRunning {
		return
	}

	// restart a task if it fails
	if t.Status.State > api.TaskStateRunning {
		o.restartTasks[t.ID] = struct{}{}
	}
}

// handleTaskChange defines what orchestrator does when a task is updated by agent
func (o *Orchestrator) handleTaskChange(ctx context.Context, t *api.Task) {
	if _, exists := o.services[t.ServiceID]; !exists {
		return
	}
	// if a task's DesiredState has passed running, it
	// means the task has been processed
	if t.DesiredState > api.TaskStateRunning {
		return
	}

	// if a task has passed running, restart it
	if t.Status.State > api.TaskStateRunning {
		o.restartTasks[t.ID] = struct{}{}
	}
}

// Stop stops the orchestrator.
func (o *Orchestrator) Stop() {
	close(o.stopChan)
	<-o.doneChan
	o.updater.CancelAll()
	o.restarts.CancelAll()
}

func (o *Orchestrator) foreachTaskFromNode(ctx context.Context, node *api.Node, cb func(context.Context, *store.Batch, *api.Task)) {
	var (
		tasks []*api.Task
		err   error
	)
	o.store.View(func(tx store.ReadTx) {
		tasks, err = store.FindTasks(tx, store.ByNodeID(node.ID))
	})
	if err != nil {
		log.G(ctx).WithError(err).Errorf("static orchestrator: foreachTaskFromNode failed finding tasks")
		return
	}

	err = o.store.Batch(func(batch *store.Batch) error {
		for _, t := range tasks {
			// Static orchestrator only removes tasks from staticServices
			if _, exists := o.services[t.ServiceID]; exists {
				cb(ctx, batch, t)
			}
		}
		return nil
	})
	if err != nil {
		log.G(ctx).WithError(err).Errorf("static orchestrator: foreachTaskFromNode failed batching tasks")
	}
}

func (o *Orchestrator) reconcileServices(ctx context.Context, serviceIDs []string) {
	updateTasksByService := make(map[string]orchestrator.Slot)
	needsTasks := []*api.Service{}

	o.store.View(func(tx store.ReadTx) {
		for _, serviceID := range serviceIDs {
			service := o.services[serviceID]
			if service == nil {
				continue
			}

			// These tasks should all be on the same node.
			tasks, err := store.FindTasks(tx, store.ByServiceID(serviceID))
			if err != nil {
				log.G(ctx).WithError(err).Errorf("static orchestrator: reconcileServices failed finding tasks for service %s", serviceID)
				continue
			}

			if len(tasks) == 0 {
				// This static service has no tasks yet! Need to create them.
				needsTasks = append(needsTasks, service)
				continue
			}

			// Keep all runnable instances of this service, and instances that
			// were not be restarted due to restart policy but may be updated
			// if the service spec changed.
			updatable := o.restarts.UpdatableTasksInSlot(ctx, tasks, service)
			if len(updatable) > 0 {
				updateTasksByService[serviceID] = updatable
			}
		}
	})

	err := o.store.Batch(func(batch *store.Batch) error {
		for _, service := range needsTasks {
			o.addTask(ctx, batch, service, service.StaticInfo.NodeID)
		}
		return nil
	})
	if err != nil {
		log.G(ctx).WithError(err).Errorf("static orchestrator: reconcileServices transaction failed")
	}

	for serviceID, updateSlot := range updateTasksByService {
		o.updater.Update(ctx, o.cluster, o.services[serviceID], []orchestrator.Slot{updateSlot})
	}
}

// updateService updates o.staticServices based on the current service value.
func (o *Orchestrator) updateService(service *api.Service) {
	o.services[service.ID] = service

	nodeID := service.StaticInfo.NodeID
	if nodeID != "" {
		if o.servicesByNodeID[nodeID] == nil {
			o.servicesByNodeID[nodeID] = make(map[string]struct{})
		}
		o.servicesByNodeID[nodeID][service.ID] = struct{}{}
	}
}

// tickTasks runs upon initialization of this orchestrator and once at the end
// of each iteration of the Run() loop.
func (o *Orchestrator) tickTasks(ctx context.Context) {
	if len(o.restartTasks) == 0 {
		return
	}
	err := o.store.Batch(func(batch *store.Batch) error {
		for taskID := range o.restartTasks {
			err := batch.Update(func(tx store.Tx) error {
				// Ensure the task still exists and is not yet 'complete'.
				t := store.GetTask(tx, taskID)
				if t == nil || t.DesiredState > api.TaskStateRunning {
					return nil
				}

				// Ensure that the service still exists.
				service := store.GetService(tx, t.ServiceID)
				if service == nil {
					return nil
				}

				return o.restarts.Restart(ctx, tx, o.cluster, service, *t)
			})
			if err != nil {
				log.G(ctx).WithError(err).Errorf("orchestrator restartTask transaction failed")
			}
		}
		return nil
	})
	if err != nil {
		log.G(ctx).WithError(err).Errorf("static orchestrator: restartTask transaction failed")
	}
	o.restartTasks = make(map[string]struct{})
}

func (o *Orchestrator) addTask(ctx context.Context, batch *store.Batch, service *api.Service, nodeID string) {
	task := orchestrator.NewTask(o.cluster, service, 0, nodeID)
	// Inject a materialized peer group config reference.
	task.MaterializedConfigs = append(task.MaterializedConfigs, orchestrator.PeerGroupConfigRef(task))

	err := batch.Update(func(tx store.Tx) error {
		if store.GetService(tx, service.ID) == nil {
			return nil
		}
		return store.CreateTask(tx, task)
	})
	if err != nil {
		log.G(ctx).WithError(err).Errorf("static orchestrator: failed to create task")
	}
}

func (o *Orchestrator) deleteTask(ctx context.Context, batch *store.Batch, t *api.Task) {
	err := batch.Update(func(tx store.Tx) error {
		return store.DeleteTask(tx, t.ID)
	})
	if err != nil {
		log.G(ctx).WithError(err).Errorf("static orchestrator: deleteTask failed to delete %s", t.ID)
	}
}

// IsRelatedService returns true if the service should be governed by this
// orchestrator. We only handle static services which have been assigned a node
// ID by the scheduler.
func (o *Orchestrator) IsRelatedService(service *api.Service) bool {
	return orchestrator.IsStaticService(service) && service.StaticInfo.NodeID != ""
}

// SlotTuple returns a slot tuple for the static service task.
func (o *Orchestrator) SlotTuple(t *api.Task) orchestrator.SlotTuple {
	return orchestrator.SlotTuple{
		ServiceID: t.ServiceID,
		NodeID:    t.NodeID,
	}
}
