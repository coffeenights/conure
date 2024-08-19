package templates

import (
	corev1 "k8s.io/api/core/v1"
	"strconv"
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
	  if #config.network.type == "public" {
		  type: corev1.#ServiceTypeLoadBalancer
		}
		if #config.network.type == "private" {
		  type: corev1.#ServiceTypeClusterIP
		}

		selector: #config.selector.labels
		ports: [ for item in #config.network.ports {
      port:       strconv.Atoi(item.hostPort)
      targetPort: strconv.Atoi(item.targetPort)
      protocol:   item.protocol
      name:       "http"
    }]
	}
}
