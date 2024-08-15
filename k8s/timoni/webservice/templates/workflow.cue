package templates

import (
	conurev1 "conure.io/apis/core/v1alpha1"
)


#ComponentWorkflow: conurev1.#Workflow & {
	#config: #Config
	apiVersion: "core.conure.io/v1alpha1"
	kind:       "Workflow"
	metadata: #config.metadata
	spec: conurev1.#WorkflowSpec & {
		actions: [
			if #config.sourceSettings.type == "git" {
				if #config.sourceSettings.buildTool == "dockerfile" {
					{
						name: "build-image",
						values: {
							branch: #config.sourceSettings.gitBranch
							dockerFile: #config.sourceSettings.dockerfilePath
							gitRepository: #config.sourceSettings.gitRepository
							imagePullSecretsName: #config.sourceSettings.imagePullSecretsName
							ociRepository: #config.sourceSettings.ociRepository
							ociTag: "latest"
						}
					},
				}
			}
		]
	}
}