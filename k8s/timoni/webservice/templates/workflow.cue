package templates

import (
	conurev1 "conure.io/apis/core/v1alpha1"
)


#ComponentWorkflow: conurev1.#Workflow & {
	#config: #Config
	spec: conurev1.#WorkflowSpec & {
		actions: [
			{
				name: "build-image",
				values: {
					gitRepository: #config.ociRepository
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