package providers

import (
	"fmt"
	k8sUtils "github.com/coffeenights/conure/internal/k8s"
	"testing"
)

func TestProviderStatusVela_GetActivity(t *testing.T) {
	providerStatusVela, _ := NewProviderStatusVela("65d6db08a7d5cf185f75e6d2", "65f91a8bfff40488c9329dcc", "9f14717c-development")
	err := providerStatusVela.GetActivity("backend-service")
	if err != nil {
		t.Fatal(err)
	}
}

func TestProviderStatusVela_GetApplicationByLabels(t *testing.T) {
	clientset, err := k8sUtils.GetClientset()
	if err != nil {
		t.Errorf("Error getting clientset: %v\n", err)
	}
	filter := map[string]string{
		OrganizationIDLabel: "65d6db08a7d5cf185f75e6d2",
		ApplicationIDLabel:  "65f91a8bfff40488c9329dcc",
	}

	velaApplication, err := k8sUtils.GetApplicationByLabelsNew(clientset, "9f14717c-development", filter)
	if err != nil {
		t.Errorf("Error getting application: %v\n", err)
	}
	fmt.Printf("Application: %v\n", velaApplication)
}
