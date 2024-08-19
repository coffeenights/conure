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
			if #config.source.sourceType == "git" {
				if #config.source.buildTool == "dockerfile" {
					{
						name: "build-image",
						values: {
							branch: #config.source.gitBranch
							dockerFile: #config.source.dockerfilePath
							gitRepository: #config.source.gitRepository
							imagePullSecretsName: #config.source.imagePullSecretsName
							ociRepository: #config.source.ociRepository
							ociTag: "latest"
						}
					},
				}
			}
		]
	}
}