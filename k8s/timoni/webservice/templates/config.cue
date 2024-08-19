package templates

import (
	corev1 "k8s.io/api/core/v1"
	timoniv1 "timoni.sh/core/v1alpha1"
)


#Port: {
	hostPort: string
	targetPort: string
	protocol: corev1.#Protocol
}

#Storage: {
	size: string
	name: string
	mountPath: string
}

// Config defines the schema and defaults for the Instance values.
#Config: {
	// The kubeVersion is a required field, set at apply-time
	// via timoni.cue by querying the user's Kubernetes API.
	kubeVersion!: string
	// Using the kubeVersion you can enforce a minimum Kubernetes minor version.
	// By default, the minimum Kubernetes version is set to 1.20.
	clusterVersion: timoniv1.#SemVer & {#Version: kubeVersion, #Minimum: "1.20.0"}

	// The moduleVersion is set from the user-supplied module version.
	// This field is used for the `app.kubernetes.io/version` label.
	moduleVersion!: string

	// The Kubernetes metadata common to all resources.
	// The `metadata.name` and `metadata.namespace` fields are
	// set from the user-supplied instance name and namespace.
	metadata: timoniv1.#Metadata & {#Version: moduleVersion}

	// The labels allows adding `metadata.labels` to all resources.
	// The `app.kubernetes.io/name` and `app.kubernetes.io/version` labels
	// are automatically generated and can't be overwritten.
	metadata: labels: timoniv1.#Labels

	// The annotations allows adding `metadata.annotations` to all resources.
	metadata: annotations?: timoniv1.#Annotations

	// The selector allows adding label selectors to Deployments and Services.
	// The `app.kubernetes.io/name` label selector is automatically generated
	// from the instance name and can't be overwritten.
	selector: timoniv1.#Selector & {#Name: metadata.name}

	resources: {
		replicas: string //int & >=0
		cpu:      timoniv1.#CPUQuantity
		memory:   timoniv1.#MemoryQuantity
	}
	source: {
		sourceType: "git" | "oci"
		if sourceType == "git" {
        gitRepository: string
        gitBranch: string
        buildTool: "nixpack" | *"dockerfile"
				if buildTool == "dockerfile" {
						dockerfilePath: string
				}
				if buildTool == "nixpack" {
						nixpackPath: string
				}
				ociRepository: "registry-service.conure-system.svc.cluster.local:5000/services/" + metadata.name
				tag: string
    }
    if sourceType == "oci" {
        ociRepository: string
        tag: string
    }
		command: [...string]
		workingDir: string
		imagePullSecretsName: string
	}
	network: {
		exposed: bool
		type: *"public" | "private"
		ports: [...#Port]
	}
	storage?: [...#Storage]
}

// Instance takes the config values and outputs the Kubernetes objects.
#Instance: {
	config: #Config

	objects: {
			workflow: #ComponentWorkflow & {#config: config}
			deploy: #Deployment & {#config: config}
			service: #Service & {#config: config}
			for index, value in config.storage {
				"\(config.metadata.name)-pvc-\(index)": #PVC & {#config: config, #index: index, #value: value}
			}
	}
}
