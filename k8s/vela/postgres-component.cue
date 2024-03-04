package postgres

import "encoding/base64"

"postgres": {
	alias: ""
	annotations: {}
	attributes: workload: definition: {
		apiVersion: "apps/v1"
		kind:       "StatefulSet"
	}
	description: "postgres statefulset using docker images"
	labels: {}
	type: "component"
}

template: {
	output: {
		apiVersion: "apps/v1"
		kind:       "StatefulSet"
		metadata: name: parameter.name
		spec: {
			replicas: 1
			selector: matchLabels: app: parameter.name
			serviceName: parameter.name
			template: {
				metadata: labels: app: parameter.name
				spec: containers: [{
					env: [{
						name: "POSTGRES_PASSWORD"
						valueFrom: secretKeyRef: {
							key:  "password"
							name: "postgres-secret"
						}
					}]
					image: parameter.image
					name:  "postgres"
					ports: [{
						containerPort: 5432
					}]
					volumeMounts: [{
						mountPath: "/var/lib/postgresql/data"
						name:      "postgres-data"
					}]
				}]
			}
			volumeClaimTemplates: [{
				metadata: name: "postgres-data"
				spec: {
					accessModes: ["ReadWriteOnce"]
					resources: requests: storage: parameter.storage
				}
			}]
		}
	}
	outputs: "postgres-secret": {
		apiVersion: "v1"
		data: password: base64.Encode(null, parameter.password)
		kind: "Secret"
		metadata: name: "postgres-secret"
		type: "Opaque"
	}
	parameter: {
		name: string
		password: string
		storage: string
		image?: *"postgres:latest" | string
	}
}
