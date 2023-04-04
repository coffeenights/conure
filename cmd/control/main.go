package main

import (
	"context"
	"log"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	core_oam_dev "github.com/oam-dev/kubevela/apis/core.oam.dev"
	"github.com/oam-dev/kubevela/apis/core.oam.dev/common"
	core_v1beta1 "github.com/oam-dev/kubevela/apis/core.oam.dev/v1beta1"
	kubevelaapistandard "github.com/oam-dev/kubevela/apis/standard.oam.dev/v1alpha1"
	"github.com/oam-dev/kubevela/pkg/oam/util"
)

var scheme = runtime.NewScheme()

func init() {
	_ = core_oam_dev.AddToScheme(scheme)
	_ = kubevelaapistandard.AddToScheme(scheme)
}

func main() {
	k8sClient, err := client.New(ctrl.GetConfigOrDie(), client.Options{Scheme: scheme})
	if err != nil {
		log.Fatal(err)
	}

	err = k8sClient.Create(context.Background(), &core_v1beta1.Application{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Application",
			APIVersion: "core.oam.dev/v1beta1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "second-vela-app",
			Namespace: "dev",
		},
		Spec: core_v1beta1.ApplicationSpec{
			Components: []common.ApplicationComponent{
				{
					Name: "express-server",
					Type: "webservice",
					Properties: util.Object2RawExtension(map[string]interface{}{
						"image": "oamdev/hello-world",
						"ports": []map[string]interface{}{
							{
								"port":   8000,
								"expose": true,
							},
						},
					}),
					Traits: []common.ApplicationTrait{
						{
							Type: "scaler",
							Properties: util.Object2RawExtension(map[string]interface{}{
								"replicas": 1,
							}),
						},
					},
				},
			},
			Policies: []core_v1beta1.AppPolicy{
				{
					Name: "target-default",
					Type: "topology",
					Properties: util.Object2RawExtension(map[string]interface{}{
						"clusters":  []string{"local"},
						"namespace": "default",
					}),
				},
				{
					Name: "target-prod",
					Type: "topology",
					Properties: util.Object2RawExtension(map[string]interface{}{
						"clusters":  []string{"local"},
						"namespace": "prod",
					}),
				}, {
					Name: "deploy-ha",
					Type: "override",
					Properties: util.Object2RawExtension(map[string]interface{}{
						"components": []map[string]interface{}{
							{
								"type": "webservice",
								"traits": []map[string]interface{}{
									{
										"type": "scaler",
										"properties": map[string]interface{}{
											"replicas": 2,
										},
									},
								},
							},
						},
					}),
				},
			},
			Workflow: &core_v1beta1.Workflow{
				Steps: []core_v1beta1.WorkflowStep{
					{
						Name: "deploy2default",
						Type: "deploy",
						Properties: util.Object2RawExtension(map[string]interface{}{
							"policies": []string{"target-default"},
						}),
					},
					{
						Name: "manual-approval",
						Type: "suspend",
					},
					{
						Name: "deploy2prod",
						Type: "deploy",
						Properties: util.Object2RawExtension(map[string]interface{}{
							"policies": []string{"target-prod", "deploy-ha"},
						}),
					},
				},
			},
		},
	})

	if err != nil {
		log.Fatal(err)
	}
}
