package workflow

import (
	k8sUtils "github.com/coffeenights/conure/internal/k8s"
)

func RunAction() error {
	_, err := k8sUtils.GetClientset()
	if err != nil {
		return err
	}

	return nil
}
