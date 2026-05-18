// Package credentials is the org-scoped credential API: the MongoDB source of
// truth for registry/git credentials a deploy later projects into Kubernetes
// Secrets. It mirrors the settings-integration handler shape (org param ->
// org.GetById -> CanWrite/CanReadOrg -> AES-encrypt material via keyStorage)
// and the variables encryption model: the stored Secret is always ciphertext,
// and listing returns metadata only — material never leaves this package in a
// response.
package credentials

import (
	"errors"
	"log"
	"net/http"
	"regexp"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"

	apiConfig "github.com/coffeenights/conure/cmd/api-server/config"
	"github.com/coffeenights/conure/cmd/api-server/conureerrors"
	"github.com/coffeenights/conure/cmd/api-server/database"
	"github.com/coffeenights/conure/cmd/api-server/middlewares"
	"github.com/coffeenights/conure/cmd/api-server/models"
	"github.com/coffeenights/conure/cmd/api-server/variables"
)

// credentialNameRE is the allowed logical-credential-name format: a single
// RFC1123 DNS label (lowercase alphanumeric, '-' allowed interior, 1–63
// chars). It is deliberately stricter than "non-empty" for two structural
// reasons, both of which are silent corruption otherwise:
//
//   - The delete route is DELETE /:organizationID/c/:name, a single Gin path
//     segment. A name with '/' (or other path metacharacters) creates a
//     credential that can never be addressed for delete.
//   - SecretName(orgID, credName) runs the name through sanitize() (maps every
//     char outside [a-z0-9-] to '-', lowercases). Distinct logical names like
//     "my.cred" and "my/cred" collapse to the same projected Secret, so one
//     credential's material overwrites another's in conure-system.
//
// Constraining the name to this subset at create/rotate makes it both a safe
// URL path segment and stable through sanitize() (sanitize is the identity on
// this set), so neither failure mode is reachable.
var credentialNameRE = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)

type ApiHandler struct {
	MongoDB    *database.MongoDB
	Config     *apiConfig.Config
	keyStorage variables.SecretKeyStorage
}

func NewApiHandler(config *apiConfig.Config, mongo *database.MongoDB,
	keyStorage variables.SecretKeyStorage) *ApiHandler {
	return &ApiHandler{
		MongoDB:    mongo,
		Config:     config,
		keyStorage: keyStorage,
	}
}

// CreateCredentialRequest is the developer/admin-facing input. Secret is the
// raw password/token — it is encrypted before storage and never echoed back.
// The CLI sends it from stdin so it never lands in shell history; the API
// still accepts it as a normal JSON field.
type CreateCredentialRequest struct {
	Name string `json:"name" binding:"required"`
	Kind string `json:"kind" binding:"required"`
	// RegistryURL/Username are non-secret metadata. Username defaults to
	// "x-access-token" for git when omitted (GitHub/GitLab token convention).
	RegistryURL string `json:"registry_url"`
	Username    string `json:"username"`
	Secret      string `json:"secret" binding:"required"`
}

// CredentialResponse is the metadata-only view. It deliberately has no field
// for the secret material: list/get can never leak it regardless of what the
// stored model carries.
type CredentialResponse struct {
	ID             string `json:"id"`
	OrganizationID string `json:"organization_id"`
	Name           string `json:"name"`
	Kind           string `json:"kind"`
	RegistryURL    string `json:"registry_url,omitempty"`
	Username       string `json:"username,omitempty"`
	CreatedAt      string `json:"created_at"`
	UpdatedAt      string `json:"updated_at"`
}

func toResponse(c *models.Credential) CredentialResponse {
	return CredentialResponse{
		ID:             c.ID.Hex(),
		OrganizationID: c.OrganizationID.Hex(),
		Name:           c.Name,
		Kind:           string(c.Kind),
		RegistryURL:    c.RegistryURL,
		Username:       c.Username,
		CreatedAt:      c.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:      c.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

// loadOrgForWrite resolves the org from the route and enforces write access.
// Returns the loaded org and true on success; on failure it has already
// written the error response and the caller must return.
func (a *ApiHandler) loadOrgForWrite(c *gin.Context) (*models.Organization, bool) {
	if _, err := primitive.ObjectIDFromHex(c.Param("organizationID")); err != nil {
		conureerrors.AbortWithError(c, conureerrors.ErrInvalidRequest)
		return nil, false
	}
	org := &models.Organization{}
	if _, err := org.GetById(a.MongoDB, c.Param("organizationID")); err != nil {
		conureerrors.AbortWithError(c, conureerrors.ErrObjectNotFound)
		return nil, false
	}
	user := c.MustGet("currentUser").(models.User)
	if !middlewares.CanWriteOrg(user, org) {
		conureerrors.AbortWithError(c, conureerrors.ErrNotAllowed)
		return nil, false
	}
	return org, true
}

// CreateCredential upserts by (org, name): posting an existing name replaces
// it in place (rotation), otherwise it inserts. This matches the
// component-definition set semantics and keeps `conure credential set`
// idempotent without a separate update verb.
func (a *ApiHandler) CreateCredential(c *gin.Context) {
	org, ok := a.loadOrgForWrite(c)
	if !ok {
		return
	}

	var req CreateCredentialRequest
	if err := c.BindJSON(&req); err != nil {
		conureerrors.AbortWithError(c, conureerrors.ErrInvalidRequest)
		return
	}
	if !validateCredential(c, &req) {
		return
	}

	encrypted, err := variables.EncryptValue(a.keyStorage, req.Secret)
	if err != nil {
		conureerrors.AbortWithError(c, conureerrors.ErrCryptoError)
		return
	}

	cred := &models.Credential{}
	err = cred.GetByOrgAndName(c.Request.Context(), a.MongoDB, org.ID, req.Name)
	switch {
	case err == nil:
		// Rotate in place: preserve identity/createdAt, replace material.
		cred.Kind = models.CredentialKind(req.Kind)
		cred.RegistryURL = req.RegistryURL
		cred.Username = req.Username
		cred.Secret = encrypted
		if uErr := cred.Update(c.Request.Context(), a.MongoDB); uErr != nil {
			conureerrors.AbortWithError(c, uErr)
			return
		}
		c.JSON(http.StatusOK, toResponse(cred))
	case errors.Is(err, conureerrors.ErrObjectNotFound):
		cred = &models.Credential{
			OrganizationID: org.ID,
			Name:           req.Name,
			Kind:           models.CredentialKind(req.Kind),
			RegistryURL:    req.RegistryURL,
			Username:       req.Username,
			Secret:         encrypted,
		}
		if _, cErr := cred.Create(c.Request.Context(), a.MongoDB); cErr != nil {
			conureerrors.AbortWithError(c, cErr)
			return
		}
		c.JSON(http.StatusCreated, toResponse(cred))
	default:
		conureerrors.AbortWithError(c, err)
	}
}

func (a *ApiHandler) ListCredentials(c *gin.Context) {
	if _, err := primitive.ObjectIDFromHex(c.Param("organizationID")); err != nil {
		conureerrors.AbortWithError(c, conureerrors.ErrInvalidRequest)
		return
	}
	org := &models.Organization{}
	if _, err := org.GetById(a.MongoDB, c.Param("organizationID")); err != nil {
		conureerrors.AbortWithError(c, conureerrors.ErrObjectNotFound)
		return
	}
	if !middlewares.CanReadOrg(c.MustGet("currentUser").(models.User), org) {
		conureerrors.AbortWithError(c, conureerrors.ErrNotAllowed)
		return
	}

	cred := &models.Credential{}
	creds, err := cred.ListByOrg(c.Request.Context(), a.MongoDB, org.ID)
	if err != nil {
		log.Printf("Error listing credentials: %v\n", err)
		conureerrors.AbortWithError(c, conureerrors.ErrInternalError)
		return
	}
	out := make([]CredentialResponse, 0, len(creds))
	for i := range creds {
		out = append(out, toResponse(&creds[i]))
	}
	c.JSON(http.StatusOK, out)
}

func (a *ApiHandler) DeleteCredential(c *gin.Context) {
	org, ok := a.loadOrgForWrite(c)
	if !ok {
		return
	}
	name := c.Param("name")
	cred := &models.Credential{}
	if err := cred.GetByOrgAndName(c.Request.Context(), a.MongoDB, org.ID, name); err != nil {
		conureerrors.AbortWithError(c, err)
		return
	}
	if err := cred.Delete(c.Request.Context(), a.MongoDB); err != nil {
		log.Printf("Error deleting credential: %v\n", err)
		conureerrors.AbortWithError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// validateCredential enforces the kind enum and kind-specific required
// material. ghcr.io has two hard constraints worth failing early on (and
// surfacing in the message) because they are the most common cause of an
// otherwise-silent pull/push 403: it rejects fine-grained PATs (classic PAT
// with write:packages only) and requires a non-empty username matching the
// PAT owner.
func validateCredential(c *gin.Context, req *CreateCredentialRequest) bool {
	if len(req.Name) > 63 || !credentialNameRE.MatchString(req.Name) {
		conureerrors.AbortWithError(c, conureerrors.WithDetail(
			conureerrors.ErrFieldValidation,
			"credential name %q is invalid: use 1–63 lowercase alphanumeric "+
				"characters or '-' (must start and end alphanumeric), e.g. "+
				"\"ghcr\" or \"my-registry\"", req.Name))
		return false
	}
	kind := models.CredentialKind(req.Kind)
	if !kind.IsValid() {
		conureerrors.AbortWithError(c, conureerrors.ErrFieldValidation)
		return false
	}
	switch kind {
	case models.CredentialKindRegistry:
		// Username is required for every registry, not just ghcr.io: ghcr
		// rejects an empty user outright and the dockerconfigjson we project
		// needs a concrete user:pass pair. (ghcr also rejects fine-grained
		// PATs — classic PAT with write:packages only — but that is not
		// statically detectable from the token, so it stays a doc concern.)
		if req.RegistryURL == "" || req.Username == "" {
			conureerrors.AbortWithError(c, conureerrors.ErrFieldValidation)
			return false
		}
	case models.CredentialKindGit:
		// Username is optional for git; default applied at projection time.
	}
	return true
}
