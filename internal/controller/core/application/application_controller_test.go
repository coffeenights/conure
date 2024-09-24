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
	Context("When the application is created", func() {
		const (
			ApplicationName      = "test-application"
			ApplicationNamespace = "default"
			ComponentName        = "test-component"
			ComponentType        = "webservice"
			timeout              = time.Second * 10
			interval             = time.Millisecond * 250
		)
		var (
			componentValues conurev1alpha1.Values
			application     *conurev1alpha1.Application
		)
		ctx := context.Background()

		BeforeEach(func() {
			componentValues = conurev1alpha1.Values{
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
			application = &conurev1alpha1.Application{
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
								ComponentType: ComponentType,
								OCIRepository: "oci://dev.conure.local:30050/components/webservice",
								OCITag:        "latest",
								Values:        componentValues,
							},
						},
					},
				},
				Status: conurev1alpha1.ApplicationStatus{},
			}
		})

		It("creates an application in k8s", func() {
			Expect(k8sClient.Create(ctx, application)).Should(Succeed())
			createdApplication := &conurev1alpha1.Application{}
			lk := types.NamespacedName{Name: ApplicationName, Namespace: ApplicationNamespace}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, lk, createdApplication)
				return err == nil
			}, timeout, interval).Should(BeTrue())
		})
		It("creates a component in k8s", func() {
			createdComponent := &conurev1alpha1.Component{}
			Eventually(func() bool {
				lk := types.NamespacedName{Name: ComponentName, Namespace: ApplicationNamespace}
				err := k8sClient.Get(ctx, lk, createdComponent)
				return err == nil
			}, timeout, interval).Should(BeTrue())
		})
		It("creates a workflow in k8s", func() {
			createdWorkflow := &conurev1alpha1.Workflow{}
			Eventually(func() bool {
				lk := types.NamespacedName{Name: ComponentType, Namespace: ApplicationNamespace}
				err := k8sClient.Get(ctx, lk, createdWorkflow)
				return err == nil
			}, timeout, interval).Should(BeTrue())
		})

		//By("Updating the component status")
		//createdComponent.TypeMeta.APIVersion = conurev1alpha1.GroupVersion.String()
		//createdComponent.TypeMeta.Kind = "Component"
		//createdComponent.Status.Conditions = []v1.Condition{
		//	{
		//		Type:   conurev1alpha1.ComponentConditionTypeReady.String(),
		//		Status: v1.ConditionTrue,
		//		Reason: conurev1alpha1.ComponentReadyDeployingReason.String(),
		//		LastTransitionTime: v1.Time{
		//			Time: time.Now(),
		//		},
		//		Message: "Test",
		//	},
		//}
		//Expect(k8sClient.Status().Update(ctx, createdComponent)).Should(Succeed())
		//
		//Eventually(func() bool {
		//	var wflw conurev1alpha1.Workflow
		//	lk = types.NamespacedName{Name: createdComponent.Spec.ComponentType, Namespace: ApplicationNamespace}
		//	err := k8sClient.Get(ctx, lk, &wflw)
		//	return err == nil
		//}, timeout, interval).Should(BeTrue())
		//
		//Eventually(func() bool {
		//	lk = types.NamespacedName{Name: ApplicationName, Namespace: ApplicationNamespace}
		//	k8sClient.Get(ctx, lk, createdApplication)
		//	return createdApplication.Status.TotalComponents == 1
		//}, timeout, interval).Should(BeTrue())
		//})
	})
})
