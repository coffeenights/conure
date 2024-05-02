"mongo": {
	alias: ""
	annotations: {}
	attributes: workload: {
		definition: {
			apiVersion: "apps/v1"
			kind:       "StatefulSet"
		}
		type: "statefulsets.apps"
	}
	description: "Mongo DB component statefulset"
	labels: {}
	type: "component"
}

template: {
	output: {
		apiVersion: "apps/v1"
		kind:       "StatefulSet"
		metadata: {
			labels: {
				if parameter.labels != _|_ {
					parameter.labels
				}
				if parameter.addRevisionLabel {
					"app.oam.dev/revision": context.revision
				}
				"app.oam.dev/name":      context.appName
				"app.oam.dev/component": context.name
			}
			if parameter.annotations != _|_ {
				annotations: parameter.annotations
			}
		}
		spec: {
			replicas: parameter.replicas
			selector: matchLabels: {
				"app.oam.dev/component": context.name
			}
			template: {
				metadata: {
					labels: {
						if parameter.labels != _|_ {
							parameter.labels
						}
						if parameter.addRevisionLabel {
							"app.oam.dev/revision": context.revision
						}
						"app.oam.dev/name":      context.appName
						"app.oam.dev/component": context.name
					}
					if parameter.annotations != _|_ {
						annotations: parameter.annotations
					}
				}

				spec: containers: [{
					image: parameter.image
					imagePullPolicy: parameter.imagePullPolicy
					name:  context.name
					ports: [{
						containerPort: 27017
					}]
					volumeMounts: [{
						mountPath: "/data/db"
						_name: context.name + "-data"
						name: *_name | string
					}]
					if parameter["cpu"] != _|_ {
						resources: {
							limits: cpu:   parameter.cpu
							requests: cpu: parameter.cpu
						}
					}

					if parameter["memory"] != _|_ {
						resources: {
							limits: memory:   parameter.memory
							requests: memory: parameter.memory
						}
					}
				}]
			}
			volumeClaimTemplates: [{
				_name: context.name + "-data"
				metadata: name: *_name | string
				spec: {
					accessModes: ["ReadWriteOnce"]
					resources: requests: storage: parameter.storage
				}
			}]
		}
	}
	outputs: {}
	parameter: {
		// +usage=Specify the labels in the workload
		labels?: [string]: string

		// +usage=If addRevisionLabel is true, the revision label will be added to the underlying pods
		addRevisionLabel: *true | bool

		// +usage=Specify the annotations in the workload
		annotations?: [string]: string

		// +usage=Specify the number of replicas for your service
		replicas: *1 | int

		// +usage=Specify the storage size for your service
		storage: string

		// +usage=Which image would you like to use for your service (default: mongo:latest)
		// +short=i
		image: *"mongo:latest" | string

		// +usage=Specify image pull policy for your service
		imagePullPolicy: *"Always" | "Never" | "IfNotPresent"

		// +usage=Number of CPU units for the service, like `0.5` (0.5 CPU core), `1` (1 CPU core)
		cpu?: string

		// +usage=Specifies the attributes of the memory resource required for the container.
		memory?: string
	}
}
