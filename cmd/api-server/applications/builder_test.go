package applications

import (
	"testing"

	"go.mongodb.org/mongo-driver/bson/primitive"

	conurev1alpha1 "github.com/coffeenights/conure/apis/core/v1alpha1"
	"github.com/coffeenights/conure/cmd/api-server/models"
)

// TestBuildComponentCRD_PropagatesEngine asserts that the engine selected at
// component-create time travels into the rendered Component CRD's
// spec.engine field, so the controller's (type, engine) lookup can pick the
// right ComponentDefinition on the cluster side.
func TestBuildComponentCRD_PropagatesEngine(t *testing.T) {
	app := &models.Application{Model: models.Model{ID: primitive.NewObjectID()}, OrganizationID: primitive.NewObjectID()}
	env := &models.Environment{ID: "abc12345", Name: "prod"}

	cases := []struct {
		name       string
		engine     string
		wantEngine conurev1alpha1.ComponentEngine
	}{
		{"helm engine propagates", "helm", conurev1alpha1.EngineHelm},
		{"timoni engine propagates", "timoni", conurev1alpha1.EngineTimoni},
		{"empty engine stays empty (controller defaults to timoni)", "", conurev1alpha1.ComponentEngine("")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			comp := &models.Component{
				Model:  models.Model{ID: primitive.NewObjectID()},
				Name:   "web",
				Type:   "webservice",
				Engine: tc.engine,
			}
			crd, err := BuildComponentCRD(app, env, comp, map[string]interface{}{})
			if err != nil {
				t.Fatalf("BuildComponentCRD: %v", err)
			}
			if crd.Spec.Engine != tc.wantEngine {
				t.Fatalf("Spec.Engine = %q, want %q", crd.Spec.Engine, tc.wantEngine)
			}
			if crd.Spec.ComponentType != "webservice" {
				t.Fatalf("Spec.ComponentType = %q, want webservice", crd.Spec.ComponentType)
			}
		})
	}
}
