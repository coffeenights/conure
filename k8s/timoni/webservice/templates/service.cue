package templates

import (
	corev1 "k8s.io/api/core/v1"
)

#Service: corev1.#Service & {
	#config:    #Config
	apiVersion: "v1"
	kind:       "Service"
	metadata:   #config.metadata
	if #config.service.annotations != _|_ {
		metadata: annotations: #config.service.annotations
	}
	spec: corev1.#ServiceSpec & {
	  if #config.service.type == "Public" {
		  type: corev1.#ServiceTypeLoadBalancer
		}
		if #config.service.type == "Private" {
		  type: corev1.#ServiceTypeClusterIP
		}

		selector: #config.selector.labels
		ports: [ for item in #config.network.ports {
      port:       item.hostPort
      protocol:   item.protocol
      name:       "http"
      targetPort: item.containerPort
    }]
	}
}
