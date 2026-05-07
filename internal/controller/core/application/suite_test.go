package application

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	conurev1alpha1 "github.com/coffeenights/conure/apis/core/v1alpha1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

var cfg *rest.Config
var k8sClient client.Client
var testEnv *envtest.Environment
var cancel context.CancelFunc
var ctx context.Context

func TestMain(m *testing.M) {
	log.SetLogger(zap.New(zap.UseDevMode(true)))
	ctx, cancel = context.WithCancel(context.TODO())

	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "..", "..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: true,
	}

	var err error
	cfg, err = testEnv.Start()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to start test environment: %v\n", err)
		os.Exit(1)
	}

	if err = conurev1alpha1.AddToScheme(scheme.Scheme); err != nil {
		fmt.Fprintf(os.Stderr, "failed to add scheme: %v\n", err)
		os.Exit(1)
	}

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create k8s client: %v\n", err)
		os.Exit(1)
	}

	k8sManager, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme:  scheme.Scheme,
		Metrics: metricsserver.Options{BindAddress: "0"},
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create manager: %v\n", err)
		os.Exit(1)
	}

	if err = (&ApplicationReconciler{
		Client: k8sManager.GetClient(),
		Scheme: k8sManager.GetScheme(),
	}).SetupWithManager(k8sManager); err != nil {
		fmt.Fprintf(os.Stderr, "failed to setup reconciler: %v\n", err)
		os.Exit(1)
	}

	go func() {
		if err := k8sManager.Start(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "failed to start manager: %v\n", err)
			os.Exit(1)
		}
	}()

	code := m.Run()

	cancel()
	_ = testEnv.Stop()
	os.Exit(code)
}
