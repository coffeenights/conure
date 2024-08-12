package templates

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

#Deployment: appsv1.#Deployment & {
	#config:    #Config
	apiVersion: "apps/v1"
	kind:       "Deployment"
	metadata:   #config.metadata
	spec: appsv1.#DeploymentSpec & {
		replicas: #config.resources.replicas
		selector: matchLabels: #config.selector.labels
		template: {
			metadata: {
				labels: #config.selector.labels
				if #config.pod.annotations != _|_ {
					annotations: #config.pod.annotations
				}
			}
			spec: corev1.#PodSpec & {
				containers: [
					{
						name: #config.metadata.name
						image: #config.source.image
						if #config.source.command != _|_ {
							command: #config.source.command
						}
						workingDir: #config.source.workingDir
						imagePullPolicy: "IfNotPresent"
						resources: {
							requests: {
								cpu: #config.resources.cpu
								memory: #config.resources.memory
							},
							limits: {
								cpu: #config.resources.cpu
								memory: #config.resources.memory
							}
						}
						if #config.storage != _|_ {
              volumes: [for item in #config.storage {
                name: item.name
                persistentVolumeClaim: {
                  claimName: item.claimName
                }
              }]
						}
					}
				]
				if #config.pod.imagePullSecrets != _|_ {
					imagePullSecrets: #config.pod.imagePullSecrets
				}
			}
		}
	}
}
