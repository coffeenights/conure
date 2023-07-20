package workload

import (
	oamconureiov1alpha1 "github.com/coffeenights/conure/api/oam/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
)

type Builder

func (d *oamconureiov1alpha1.DeploymentType) Build() error {
	return nil
}

func (d *DeploymentType) Spawn() error {
	return nil
}

func (d *DeploymentType) GetName() string {
	return d.Name
}
