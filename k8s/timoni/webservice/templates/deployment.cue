package templates

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"strconv"
)

#Deployment: appsv1.#Deployment & {
	#config:    #Config
	apiVersion: "apps/v1"
	kind:       "Deployment"
	metadata:   #config.metadata
	spec: appsv1.#DeploymentSpec & {
		replicas: strconv.Atoi(#config.resources.replicas)
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
						image: #config.sourceSettings.ociRepository
						if #config.sourceSettings.command != _|_ {
							command: #config.sourceSettings.command
						}
						workingDir: #config.sourceSettings.workingDir
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
							volumeMounts: [for item in #config.storage {
								mountPath: item.mountPath
								name: item.name
							}]
						}
					}
				]
				if #config.storage != _|_ {
					volumes: [for item in #config.storage {
						name: item.name
						persistentVolumeClaim: {
							claimName: #config.metadata.name + "-" + item.name
						}
					}]
				}
//				if #config.pod.imagePullSecrets != _|_ {
//					imagePullSecrets: #config.pod.imagePullSecrets
//				}
			}
		}
	}
}
