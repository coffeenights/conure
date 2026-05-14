package middlewares

import (
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/coffeenights/conure/cmd/api-server/models"
)

// CanReadOrg returns true when the user is allowed to view resources scoped
// to the given organization. Admins see everything; developers see their
// home organization (the one stamped on User.OrganizationID at provisioning
// time) plus organizations they personally created.
func CanReadOrg(user models.User, org *models.Organization) bool {
	if user.IsAdmin() {
		return true
	}
	if org.AccountID == user.ID {
		return true
	}
	if !user.OrganizationID.IsZero() && user.OrganizationID == org.ID {
		return true
	}
	return false
}

// CanWriteOrg gates write operations on an organization itself. Only the
// owner (AccountID) or an admin can rename/delete it. Developers in the
// organization can read but not mutate org-level settings.
func CanWriteOrg(user models.User, org *models.Organization) bool {
	if user.IsAdmin() {
		return true
	}
	return org.AccountID == user.ID
}

// CanWriteOwned is the generic "you may mutate things you created" check
// used by application/environment/component handlers. accountID is the
// AccountID stamped on the resource (or its parent application).
func CanWriteOwned(user models.User, accountID primitive.ObjectID) bool {
	if user.IsAdmin() {
		return true
	}
	return accountID == user.ID
}
