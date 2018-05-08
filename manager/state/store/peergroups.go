package store

import (
	"strings"

	"github.com/docker/swarmkit/api"
	memdb "github.com/hashicorp/go-memdb"
)

const tablePeerGroup = "peer_group"

func init() {
	register(ObjectStoreConfig{
		Table: &memdb.TableSchema{
			Name: tablePeerGroup,
			Indexes: map[string]*memdb.IndexSchema{
				indexID: {
					Name:    indexID,
					Unique:  true,
					Indexer: api.PeerGroupIndexerByID{},
				},
				indexName: {
					Name:    indexName,
					Unique:  true,
					Indexer: api.PeerGroupIndexerByName{},
				},
				indexCustom: {
					Name:         indexCustom,
					Indexer:      api.PeerGroupCustomIndexer{},
					AllowMissing: true,
				},
			},
		},
		Save: func(tx ReadTx, snapshot *api.StoreSnapshot) error {
			var err error
			snapshot.PeerGroups, err = FindPeerGroups(tx, All)
			return err
		},
		Restore: func(tx Tx, snapshot *api.StoreSnapshot) error {
			toStoreObj := make([]api.StoreObject, len(snapshot.PeerGroups))
			for i, x := range snapshot.PeerGroups {
				toStoreObj[i] = x
			}
			return RestoreTable(tx, tablePeerGroup, toStoreObj)
		},
		ApplyStoreAction: func(tx Tx, sa api.StoreAction) error {
			switch v := sa.Target.(type) {
			case *api.StoreAction_PeerGroup:
				obj := v.PeerGroup
				switch sa.Action {
				case api.StoreActionKindCreate:
					return CreatePeerGroup(tx, obj)
				case api.StoreActionKindUpdate:
					return UpdatePeerGroup(tx, obj)
				case api.StoreActionKindRemove:
					return DeletePeerGroup(tx, obj.ID)
				}
			}
			return errUnknownStoreAction
		},
	})
}

// CreatePeerGroup adds a new peer group to the store.
// Returns ErrExist if the ID is already taken.
func CreatePeerGroup(tx Tx, peerGroup *api.PeerGroup) error {
	// Ensure the name is not already in use by either an existing peer group
	// or any service.
	if tx.lookup(tablePeerGroup, indexName, strings.ToLower(peerGroup.Spec.Annotations.Name)) != nil {
		return ErrNameConflict
	}
	if tx.lookup(tableService, indexName, strings.ToLower(peerGroup.Spec.Annotations.Name)) != nil {
		return ErrNameConflict
	}

	return tx.create(tablePeerGroup, peerGroup)
}

// UpdatePeerGroup updates an existing peer group in the store.
// Returns ErrNotExist if the peer group doesn't exist.
func UpdatePeerGroup(tx Tx, peerGroup *api.PeerGroup) error {
	// Ensure the name is either not in use by any peer group or service unless
	// already used by this same PeerGroup.
	if existing := tx.lookup(tablePeerGroup, indexName, strings.ToLower(peerGroup.Spec.Annotations.Name)); existing != nil {
		if existing.GetID() != peerGroup.ID {
			return ErrNameConflict
		}
	} else if tx.lookup(tableService, indexName, strings.ToLower(peerGroup.Spec.Annotations.Name)) != nil {
		return ErrNameConflict
	}

	return tx.update(tablePeerGroup, peerGroup)
}

// DeletePeerGroup removes a peer group from the store.
// Returns ErrNotExist if the peer group doesn't exist.
func DeletePeerGroup(tx Tx, id string) error {
	return tx.delete(tablePeerGroup, id)
}

// GetPeerGroup looks up a peer group by ID.
// Returns nil if the peer group doesn't exist.
func GetPeerGroup(tx ReadTx, id string) *api.PeerGroup {
	storeObject := tx.get(tablePeerGroup, id)
	if storeObject == nil {
		return nil
	}
	return storeObject.(*api.PeerGroup)
}

// FindPeerGroups selects a set of peer groups and returns them.
func FindPeerGroups(tx ReadTx, by By) ([]*api.PeerGroup, error) {
	checkType := func(by By) error {
		switch by.(type) {
		case byName, byNamePrefix, byIDPrefix, byCustom, byCustomPrefix:
			return nil
		default:
			return ErrInvalidFindBy
		}
	}

	peerGroupList := []*api.PeerGroup{}
	appendResult := func(o api.StoreObject) {
		peerGroupList = append(peerGroupList, o.(*api.PeerGroup))
	}

	err := tx.find(tablePeerGroup, by, checkType, appendResult)
	return peerGroupList, err
}
