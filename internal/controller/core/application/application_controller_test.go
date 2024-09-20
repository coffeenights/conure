package application

import (
	"context"
	conurev1alpha1 "github.com/coffeenights/conure/apis/core/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"time"
)

var _ = Describe("Test Application Controller", func() {
	const (
		ApplicationName      = "test-application"
		ApplicationNamespace = "default"
		ComponentName        = "test-component"
		timeout              = time.Second * 20
		duration             = time.Second * 10
		interval             = time.Millisecond * 250
	)

	Context("Test Application Controller", func() {
		It("Test Application Controller", func() {
			By("Creating a new Application resource")
			ctx := context.Background()
			componentValues := conurev1alpha1.Values{
				Resources: conurev1alpha1.Resources{
					Replicas: 1,
					CPU:      "200m",
					Memory:   "256Mi",
				},
				Network: conurev1alpha1.Network{
					Exposed: true,
					Type:    "public",
					Ports: []conurev1alpha1.Port{
						{
							HostPort:   9091,
							TargetPort: 9091,
							Protocol:   "TCP",
						},
					},
				},
				Source: conurev1alpha1.Source{
					SourceType:           "git",
					GitRepository:        "https://github.com/mredvard/fastapi_demo.git",
					GitBranch:            "main",
					BuildTool:            "dockerfile",
					DockerfilePath:       "Dockerfile",
					Tag:                  "latest",
					Command:              []string{"uvicorn", "main:app", "--host", "0.0.0.0", "--port", "9091"},
					WorkingDir:           "/app",
					ImagePullSecretsName: "regcred",
				},
				Storage:  []conurev1alpha1.Storage{},
				Advanced: nil,
			}
			application := &conurev1alpha1.Application{
				TypeMeta: v1.TypeMeta{
					APIVersion: conurev1alpha1.GroupVersion.String(),
					Kind:       "Application",
				},
				ObjectMeta: v1.ObjectMeta{
					Name:      ApplicationName,
					Namespace: ApplicationNamespace,
				},
				Spec: conurev1alpha1.ApplicationSpec{
					Components: []conurev1alpha1.ComponentTemplate{
						{
							ComponentTemplateMetadata: conurev1alpha1.ComponentTemplateMetadata{
								Name:        ComponentName,
								Labels:      nil,
								Annotations: nil,
							},
							Spec: conurev1alpha1.ComponentSpec{
								ComponentType: "webservice",
								OCIRepository: "oci://dev.conure.local:30050/components/webservice",
								OCITag:        "latest",
								Values:        componentValues,
							},
						},
					},
				},
				Status: conurev1alpha1.ApplicationStatus{},
			}
			Expect(k8sClient.Create(ctx, application)).Should(Succeed())

			By("Retrieving the created Application resource")
			createdApplication := &conurev1alpha1.Application{}
			lk := types.NamespacedName{Name: ApplicationName, Namespace: ApplicationNamespace}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, lk, createdApplication)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			By("Waiting for the component to be created")
			createdComponent := &conurev1alpha1.Component{}
			lk = types.NamespacedName{Name: ComponentName, Namespace: ApplicationNamespace}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, lk, createdComponent)
				return err == nil
			}, timeout, interval).Should(BeTrue())
		})
	})
})
