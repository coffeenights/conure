package templates

import (
	"encoding/yaml"
	"uuid"

	corev1 "k8s.io/api/core/v1"
	batchv1 "k8s.io/api/batch/v1"
	timoniv1 "timoni.sh/core/v1alpha1"
)

#BuildJob: batchv1.#Job & {
	#config:    #Config
	apiVersion: "batch/v1"
	kind:       "Job"
	metadata: timoniv1.#MetaComponent & {
		#Meta:      #config.metadata
		#Component: #config.nameSuffix
	}
	metadata: annotations: timoniv1.Action.Force
	spec: batchv1.#JobSpec & {
		template: corev1.#PodTemplateSpec & {
			let _checksum = uuid.SHA1(uuid.ns.DNS, yaml.Marshal(#config))
			metadata: annotations: "timoni.sh/checksum": "\(_checksum)"
			spec: {
				initContainers: [
					{
						name:  "git-clone"
						image: "alpine/git"
						args:  ["clone", "--single-branch", "--branch", #config.branch, #config.gitRepository, "/workspace"]
						volumeMounts: [
							{
								name:      "dockerfile-storage"
								mountPath: "/workspace"
							}
						]
					}
				]
				containers: [{
					name:            "kaniko"
					image:           "gcr.io/kaniko-project/executor:latest"
					imagePullPolicy: "IfNotPresent"
					args: [
						"--insecure",
            "--skip-tls-verify",
						"--dockerfile=/workspace/\(#config.dockerFile)",
						"--context=/workspace",
						"--destination=\(#config.ociRepository):\(#config.ociTag)",
						"--cache=false"
					]
					volumeMounts: [
						{
							name:      "dockerfile-storage"
							mountPath: "/workspace"
						},
						{
							name:      "kaniko-secret"
							mountPath: "/kaniko/.docker"
						}
				  ]
				}]
				volumes: [
					{
						name: "dockerfile-storage"
						emptyDir: {
							sizeLimit: #config.storageSize
						}
					},
					{
						name: "kaniko-secret"
						secret: {
							secretName: #config.imagePullSecretsName
							items: [
								{
									key:  ".dockerconfigjson"
									path: "config.json"
								}
							]
						}
					}
				]
				imagePullSecrets: [{name: #config.imagePullSecretsName}]
				restartPolicy: "Never"
				if #config.podSecurityContext != _|_ {
					securityContext: #config.podSecurityContext
				}
				if #config.topologySpreadConstraints != _|_ {
					topologySpreadConstraints: #config.topologySpreadConstraints
				}
				if #config.affinity != _|_ {
					affinity: #config.affinity
				}
				if #config.tolerations != _|_ {
					tolerations: #config.tolerations
				}
			}
		}
		backoffLimit: 1
	}
}
