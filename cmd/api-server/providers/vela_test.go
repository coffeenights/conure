package providers

import "testing"

func TestProviderStatusVela_GetActivity(t *testing.T) {
	providerStatusVela, _ := NewProviderStatusVela("65d87e418c2db2d59c91f8c8", "65f93bcc578ec5b8020e31f5", "fbc70d63-development")
	err := providerStatusVela.GetActivity("backend-service")
	if err != nil {
		t.Fatal(err)
	}
}
