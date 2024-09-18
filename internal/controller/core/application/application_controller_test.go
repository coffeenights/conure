package application

import (
	"context"
	conurev1alpha1 "github.com/coffeenights/conure/apis/core/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Test Application Controller", func() {
	const (
		ApplicationName      = "test-application"
		ApplicationNamespace = "default"
	)

	Context("Test Application Controller", func() {
		It("Test Application Controller", func() {
			By("Test Application Controller")
			ctx := context.Background()
			application := &conurev1alpha1.Application{
				TypeMeta: v1.TypeMeta{
					APIVersion: "core.conure.io/v1alpha1",
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
								Name:        "test-component",
								Labels:      nil,
								Annotations: nil,
							},
							Spec: conurev1alpha1.ComponentSpec{
								ComponentType: "webservice",
								OCIRepository: "oci://test/service/webservice",
								OCITag:        "latest",
								Values:        conurev1alpha1.Values{},
							},
						},
					},
				},
				Status: conurev1alpha1.ApplicationStatus{},
			}
			Expect(k8sClient.Create(ctx, application)).Should(Succeed())
		})
	})
})
