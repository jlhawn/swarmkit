package orchestrator

import (
	"fmt"
	"path/filepath"
	"reflect"
	"time"

	"github.com/docker/swarmkit/api"
	"github.com/docker/swarmkit/api/defaults"
	"github.com/docker/swarmkit/identity"
	"github.com/docker/swarmkit/manager/constraint"
	"github.com/docker/swarmkit/protobuf/ptypes"
)

// NewTask creates a new task.
func NewTask(cluster *api.Cluster, service *api.Service, slot uint64, nodeID string) *api.Task {
	var logDriver *api.Driver
	if service.Spec.Task.LogDriver != nil {
		// use the log driver specific to the task, if we have it.
		logDriver = service.Spec.Task.LogDriver
	} else if cluster != nil {
		// pick up the cluster default, if available.
		logDriver = cluster.Spec.TaskDefaults.LogDriver // nil is okay here.
	}

	taskID := identity.NewID()
	task := &api.Task{
		ID:                 taskID,
		ServiceAnnotations: service.Spec.Annotations,
		Spec:               service.Spec.Task,
		SpecVersion:        service.SpecVersion,
		ServiceID:          service.ID,
		Slot:               slot,
		Status: api.TaskStatus{
			State:     api.TaskStateNew,
			Timestamp: ptypes.MustTimestampProto(time.Now()),
			Message:   "created",
		},
		Endpoint: &api.Endpoint{
			Spec: service.Spec.Endpoint.Copy(),
		},
		DesiredState: api.TaskStateRunning,
		LogDriver:    logDriver,
	}

	// In global and static mode we also set the NodeID
	if nodeID != "" {
		task.NodeID = nodeID
	}

	if service != nil && service.StaticInfo != nil {
		task.MaterializedConfigs = append(task.MaterializedConfigs, PeerGroupConfigRef(task))
	}

	// Add any certificate issuance config refs.
	task.MaterializedConfigs = append(task.MaterializedConfigs, GetCertificateIssuanceConfigRefs(task)...)

	return task
}

// PeerGroupConfigRef returns a config reference for a static service's
// materialized peer group config file.
func PeerGroupConfigRef(task *api.Task) *api.ConfigReference {
	return &api.ConfigReference{
		ConfigID:   fmt.Sprintf("%s-peer-group", task.ID),
		ConfigName: "peer-group",
		Target: &api.ConfigReference_File{
			File: &api.FileTarget{
				Name: "/run/peers",
				Mode: 0444,
				UID:  "0",
				GID:  "0",
			},
		},
	}
}

// GetCertificateIssuanceConfigRefs returns a slice of config references for
// any certificate issuances requested by a task.
func GetCertificateIssuanceConfigRefs(task *api.Task) []*api.ConfigReference {
	container := task.Spec.GetContainer()
	if container == nil {
		return nil
	}

	configRefs := make([]*api.ConfigReference, 0, 3*len(container.CertificateIssuances))
	for _, certIssuance := range container.CertificateIssuances {
		caConfigID := fmt.Sprintf("%s-%s-issue-ca", task.ID, certIssuance.CertificateAuthorityID)
		caTarget := certIssuance.Directory.Copy()
		caTarget.Name = filepath.Join(caTarget.Name, "ca.pem")

		keyConfigID := fmt.Sprintf("%s-%s-issue-key", task.ID, certIssuance.CertificateAuthorityID)
		keyTarget := certIssuance.Directory.Copy()
		keyTarget.Name = filepath.Join(keyTarget.Name, "key.pem")

		certConfigID := fmt.Sprintf("%s-%s-issue-cert", task.ID, certIssuance.CertificateAuthorityID)
		certTarget := certIssuance.Directory.Copy()
		certTarget.Name = filepath.Join(certTarget.Name, "cert.pem")

		configRefs = append(configRefs,
			&api.ConfigReference{
				ConfigID:   caConfigID,
				ConfigName: caConfigID,
				Target: &api.ConfigReference_File{
					File: caTarget,
				},
			},
			&api.ConfigReference{
				ConfigID:   keyConfigID,
				ConfigName: keyConfigID,
				Target: &api.ConfigReference_File{
					File: keyTarget,
				},
			},
			&api.ConfigReference{
				ConfigID:   certConfigID,
				ConfigName: certConfigID,
				Target: &api.ConfigReference_File{
					File: certTarget,
				},
			},
		)
	}

	return configRefs
}

// RestartCondition returns the restart condition to apply to this task.
func RestartCondition(task *api.Task) api.RestartPolicy_RestartCondition {
	restartCondition := defaults.Service.Task.Restart.Condition
	if task.Spec.Restart != nil {
		restartCondition = task.Spec.Restart.Condition
	}
	return restartCondition
}

// IsTaskDirty determines whether a task matches the given service's spec and
// if the given node satisfies the placement constraints.
// Returns false if the spec version didn't change,
// only the task placement constraints changed and the assigned node
// satisfies the new constraints, or the service task spec and the endpoint spec
// didn't change at all.
// Returns true otherwise.
// Note: for non-failed tasks with a container spec runtime that have already
// pulled the required image (i.e., current state is between READY and
// RUNNING inclusively), the value of the `PullOptions` is ignored.
func IsTaskDirty(s *api.Service, t *api.Task, n *api.Node) bool {
	// If the spec version matches, we know the task is not dirty. However,
	// if it does not match, that doesn't mean the task is dirty, since
	// only a portion of the spec is included in the comparison.
	if t.SpecVersion != nil && s.SpecVersion != nil && *s.SpecVersion == *t.SpecVersion {
		return false
	}

	// Make a deep copy of the service and task spec for the comparison.
	serviceTaskSpec := *s.Spec.Task.Copy()

	// Task is not dirty if the placement constraints alone changed
	// and the node currently assigned can satisfy the changed constraints.
	if IsTaskDirtyPlacementConstraintsOnly(serviceTaskSpec, t) && nodeMatches(s, n) {
		return false
	}

	// For non-failed tasks with a container spec runtime that have already
	// pulled the required image (i.e., current state is between READY and
	// RUNNING inclusively), ignore the value of the `PullOptions` field by
	// setting the copied service to have the same PullOptions value as the
	// task. A difference in only the `PullOptions` field should not cause
	// a running (or ready to run) task to be considered 'dirty' when we
	// handle updates.
	// See https://github.com/docker/swarmkit/issues/971
	currentState := t.Status.State
	// Ignore PullOpts if the task is desired to be in a "runnable" state
	// and its last known current state is between READY and RUNNING in
	// which case we know that the task either successfully pulled its
	// container image or didn't need to.
	ignorePullOpts := t.DesiredState <= api.TaskStateRunning &&
		currentState >= api.TaskStateReady &&
		currentState <= api.TaskStateRunning
	if ignorePullOpts && serviceTaskSpec.GetContainer() != nil && t.Spec.GetContainer() != nil {
		// Modify the service's container spec.
		serviceTaskSpec.GetContainer().PullOptions = t.Spec.GetContainer().PullOptions
	}

	return !reflect.DeepEqual(serviceTaskSpec, t.Spec) ||
		(t.Endpoint != nil && !reflect.DeepEqual(s.Spec.Endpoint, t.Endpoint.Spec))
}

// Checks if the current assigned node matches the Placement.Constraints
// specified in the task spec for Updater.newService.
func nodeMatches(s *api.Service, n *api.Node) bool {
	if n == nil {
		return false
	}

	constraints, _ := constraint.Parse(s.Spec.Task.Placement.Constraints)
	return constraint.NodeMatches(constraints, n)
}

// IsTaskDirtyPlacementConstraintsOnly checks if the Placement field alone
// in the spec has changed.
func IsTaskDirtyPlacementConstraintsOnly(serviceTaskSpec api.TaskSpec, t *api.Task) bool {
	// Compare the task placement constraints.
	if reflect.DeepEqual(serviceTaskSpec.Placement, t.Spec.Placement) {
		return false
	}

	// Update spec placement to only the fields
	// other than the placement constraints in the spec.
	serviceTaskSpec.Placement = t.Spec.Placement
	return reflect.DeepEqual(serviceTaskSpec, t.Spec)
}

// InvalidNode is true if the node is nil, down, or drained
func InvalidNode(n *api.Node) bool {
	return n == nil ||
		n.Status.State == api.NodeStatus_DOWN ||
		n.Spec.Availability == api.NodeAvailabilityDrain
}

// TasksByTimestamp sorts tasks by applied timestamp if available, otherwise
// status timestamp.
type TasksByTimestamp []*api.Task

// Len implements the Len method for sorting.
func (t TasksByTimestamp) Len() int {
	return len(t)
}

// Swap implements the Swap method for sorting.
func (t TasksByTimestamp) Swap(i, j int) {
	t[i], t[j] = t[j], t[i]
}

// Less implements the Less method for sorting.
func (t TasksByTimestamp) Less(i, j int) bool {
	iTimestamp := t[i].Status.Timestamp
	if t[i].Status.AppliedAt != nil {
		iTimestamp = t[i].Status.AppliedAt
	}

	jTimestamp := t[j].Status.Timestamp
	if t[j].Status.AppliedAt != nil {
		iTimestamp = t[j].Status.AppliedAt
	}

	if iTimestamp == nil {
		return true
	}
	if jTimestamp == nil {
		return false
	}
	if iTimestamp.Seconds < jTimestamp.Seconds {
		return true
	}
	if iTimestamp.Seconds > jTimestamp.Seconds {
		return false
	}
	return iTimestamp.Nanos < jTimestamp.Nanos
}
