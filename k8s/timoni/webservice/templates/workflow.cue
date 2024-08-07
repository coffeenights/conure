package templates

import (
	corev1 "k8s.io/api/core/v1"
	conurev1 "conure.io/apis/core/v1alpha1"
)


#ComponentWorkflow: conurev1.#Workflow & {
	#config: #Component
	spec: #WorkflowSpec & {
		actions: [
			{
				name: "build-image",
				values: {
					gitRepository: config.ociRepository
					branch: "main"
					imagePullSecretsName: "regcred"
					message: "Building Conure!"
					storageSize: "10Gi"
					namespace: "conure-system"
				}
			},
		]
	}
}