package templates

import (
	corev1 "k8s.io/api/core/v1"
	timoniv1 "timoni.sh/core/v1alpha1"
)

#PVC: corev1.#PersistentVolumeClaim & {
	#config: #Config
	#index: int
	#value: #Storage
	apiVersion: "v1"
	kind: "PersistentVolumeClaim"
	metadata:timoniv1.#MetaComponent & {
		#Meta: #config.metadata
		#Component: #value.name
	}
	spec: corev1.#PersistentVolumeClaimSpec & {
		accessModes: ["ReadWriteOnce"]
		resources: requests: storage: #value.size
	}
}

