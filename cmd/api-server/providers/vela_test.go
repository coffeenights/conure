package providers

import "testing"

func TestProviderStatusVela_GetActivity(t *testing.T) {
	providerStatusVela, _ := NewProviderStatusVela("65d87e418c2db2d59c91f8c8", "65f93bcc578ec5b8020e31f5", "fbc70d63-development")
	err := providerStatusVela.GetActivity("backend-service")
	if err != nil {
		t.Fatal(err)
	}
}

func TestProviderStatusVela_WatchApplicationStatus(t *testing.T) {
	providerStatusVela, err := NewProviderStatusVela("65d6db08a7d5cf185f75e6d2", "65f91a8bfff40488c9329dcc", "9f14717c-development")
	if err != nil {
		t.Fatal(err)
	}
	err = providerStatusVela.WatchApplicationStatus()
	if err != nil {
		t.Fatal(err)
	}
}
