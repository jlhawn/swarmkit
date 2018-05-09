package store

import (
	"strings"

	"github.com/docker/swarmkit/api"
	memdb "github.com/hashicorp/go-memdb"
)

const tableCertificateAuthority = "certificate_authority"

func init() {
	register(ObjectStoreConfig{
		Table: &memdb.TableSchema{
			Name: tableCertificateAuthority,
			Indexes: map[string]*memdb.IndexSchema{
				indexID: {
					Name:    indexID,
					Unique:  true,
					Indexer: api.CertificateAuthorityIndexerByID{},
				},
				indexName: {
					Name:    indexName,
					Unique:  true,
					Indexer: api.CertificateAuthorityIndexerByName{},
				},
				indexCustom: {
					Name:         indexCustom,
					Indexer:      api.CertificateAuthorityCustomIndexer{},
					AllowMissing: true,
				},
			},
		},
		Save: func(tx ReadTx, snapshot *api.StoreSnapshot) error {
			var err error
			snapshot.CertificateAuthorities, err = FindCAs(tx, All)
			return err
		},
		Restore: func(tx Tx, snapshot *api.StoreSnapshot) error {
			toStoreObj := make([]api.StoreObject, len(snapshot.CertificateAuthorities))
			for i, x := range snapshot.CertificateAuthorities {
				toStoreObj[i] = x
			}
			return RestoreTable(tx, tableCertificateAuthority, toStoreObj)
		},
		ApplyStoreAction: func(tx Tx, sa api.StoreAction) error {
			switch v := sa.Target.(type) {
			case *api.StoreAction_CertificateAuthority:
				obj := v.CertificateAuthority
				switch sa.Action {
				case api.StoreActionKindCreate:
					return CreateCA(tx, obj)
				case api.StoreActionKindUpdate:
					return UpdateCA(tx, obj)
				case api.StoreActionKindRemove:
					return DeleteCA(tx, obj.ID)
				}
			}
			return errUnknownStoreAction
		},
	})
}

// CreateCA adds a new certificate authority to the store.
// Returns ErrExist if the ID is already taken.
func CreateCA(tx Tx, ca *api.CertificateAuthority) error {
	// Ensure the name is not already in use by an existing CA.
	if tx.lookup(tableCertificateAuthority, indexName, strings.ToLower(ca.Spec.Annotations.Name)) != nil {
		return ErrNameConflict
	}

	return tx.create(tableCertificateAuthority, ca)
}

// UpdateCA updates an existing certificate authority in the store.
// Returns ErrNotExist if the CA doesn't exist.
func UpdateCA(tx Tx, ca *api.CertificateAuthority) error {
	// Ensure the name is either not in use by an existing CA.
	if existing := tx.lookup(tableCertificateAuthority, indexName, strings.ToLower(ca.Spec.Annotations.Name)); existing != nil {
		if existing.GetID() != ca.ID {
			return ErrNameConflict
		}
	}

	return tx.update(tableCertificateAuthority, ca)
}

// DeleteCA removes a certificat authority from the store.
// Returns ErrNotExist if the CA doesn't exist.
func DeleteCA(tx Tx, id string) error {
	return tx.delete(tableCertificateAuthority, id)
}

// GetCA looks up a certificate authority by ID.
// Returns nil if the CA doesn't exist.
func GetCA(tx ReadTx, id string) *api.CertificateAuthority {
	storeObject := tx.get(tableCertificateAuthority, id)
	if storeObject == nil {
		return nil
	}
	return storeObject.(*api.CertificateAuthority)
}

// FindCAs selects a set of certificate authorities and returns them.
func FindCAs(tx ReadTx, by By) ([]*api.CertificateAuthority, error) {
	checkType := func(by By) error {
		switch by.(type) {
		case byName, byNamePrefix, byIDPrefix, byCustom, byCustomPrefix:
			return nil
		default:
			return ErrInvalidFindBy
		}
	}

	caList := []*api.CertificateAuthority{}
	appendResult := func(o api.StoreObject) {
		caList = append(caList, o.(*api.CertificateAuthority))
	}

	err := tx.find(tableCertificateAuthority, by, checkType, appendResult)
	return caList, err
}
