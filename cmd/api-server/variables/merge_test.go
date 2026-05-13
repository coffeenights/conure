package variables

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/coffeenights/conure/cmd/api-server/models"
)

func TestMergeAllScopes_Empty(t *testing.T) {
	got := MergeAllScopes(nil, nil, nil)
	assert.Equal(t, 0, len(got))
}

func TestMergeAllScopes_PreservesType(t *testing.T) {
	org := []models.Variable{{Name: "A", Value: "org-a", Type: models.OrganizationType}}
	env := []models.Variable{{Name: "B", Value: "env-b", Type: models.EnvironmentType}}
	comp := []models.Variable{{Name: "C", Value: "comp-c", Type: models.ComponentType}}

	got := MergeAllScopes(org, env, comp)
	assert.Equal(t, 3, len(got))
	// Sorted alphabetically by name.
	assert.Equal(t, "A", got[0].Name)
	assert.Equal(t, models.OrganizationType, got[0].Type)
	assert.Equal(t, "B", got[1].Name)
	assert.Equal(t, models.EnvironmentType, got[1].Type)
	assert.Equal(t, "C", got[2].Name)
	assert.Equal(t, models.ComponentType, got[2].Type)
}

func TestMergeAllScopes_EnvOverridesOrg(t *testing.T) {
	org := []models.Variable{{Name: "SHARED", Value: "org-value", Type: models.OrganizationType}}
	env := []models.Variable{{Name: "SHARED", Value: "env-value", Type: models.EnvironmentType}}

	got := MergeAllScopes(org, env, nil)
	assert.Equal(t, 1, len(got))
	assert.Equal(t, "env-value", got[0].Value)
	assert.Equal(t, models.EnvironmentType, got[0].Type, "winning entry should reflect the env tier")
}

func TestMergeAllScopes_ComponentOverridesEnvOverridesOrg(t *testing.T) {
	org := []models.Variable{{Name: "SHARED", Value: "org-value", Type: models.OrganizationType}}
	env := []models.Variable{{Name: "SHARED", Value: "env-value", Type: models.EnvironmentType}}
	comp := []models.Variable{{Name: "SHARED", Value: "comp-value", Type: models.ComponentType}}

	got := MergeAllScopes(org, env, comp)
	assert.Equal(t, 1, len(got))
	assert.Equal(t, "comp-value", got[0].Value)
	assert.Equal(t, models.ComponentType, got[0].Type)
}

func TestMergeAllScopes_NonCollidingNames(t *testing.T) {
	org := []models.Variable{
		{Name: "ORG_ONLY", Value: "o", Type: models.OrganizationType},
		{Name: "BOTH", Value: "org", Type: models.OrganizationType},
	}
	env := []models.Variable{
		{Name: "BOTH", Value: "env", Type: models.EnvironmentType},
		{Name: "ENV_ONLY", Value: "e", Type: models.EnvironmentType},
	}

	got := MergeAllScopes(org, env, nil)
	byName := map[string]models.Variable{}
	for _, v := range got {
		byName[v.Name] = v
	}
	assert.Equal(t, 3, len(got))
	assert.Equal(t, "o", byName["ORG_ONLY"].Value)
	assert.Equal(t, "env", byName["BOTH"].Value)
	assert.Equal(t, models.EnvironmentType, byName["BOTH"].Type)
	assert.Equal(t, "e", byName["ENV_ONLY"].Value)
}

func TestMergeAllScopes_PreservesEncryptedFlag(t *testing.T) {
	// Secrets are decrypted *after* merging by the caller, so the merge
	// must preserve IsEncrypted on the winning entry — otherwise the caller
	// can't tell whether the stored Value still needs decryption.
	id := primitive.NewObjectID()
	env := []models.Variable{{ID: id, Name: "SECRET", Value: "ciphertext", IsEncrypted: true, Type: models.EnvironmentType}}

	got := MergeAllScopes(nil, env, nil)
	assert.Equal(t, 1, len(got))
	assert.True(t, got[0].IsEncrypted)
	assert.Equal(t, "ciphertext", got[0].Value)
	assert.Equal(t, id, got[0].ID)
}
