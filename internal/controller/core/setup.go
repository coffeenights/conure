package controller

import (
	"github.com/coffeenights/conure/internal/controller/core/application"
	"github.com/coffeenights/conure/internal/controller/core/workflow"
	ctrl "sigs.k8s.io/controller-runtime"
)

func Setup(mgr ctrl.Manager) error {
	// Define a slice of setup functions
	setupFunctions := []func(ctrl.Manager) error{
		application.Setup,
		workflow.Setup,
		// component.Setup,
	}
	// Iterate over each setup function
	for _, setup := range setupFunctions {
		// Call the setup function with mgr and args
		if err := setup(mgr); err != nil {
			// Return the error if any setup function fails
			return err
		}
	}
	// Return nil if all setup functions succeed
	return nil
}
