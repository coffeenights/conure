@if(debug)

package main

// Values used by debug_tool.cue.
// Debug example 'cue cmd -t debug -t name=test -t namespace=test -t mv=1.0.0 -t kv=1.28.0 build'.
values: {
	branch: "main"
	dockerFile: "cmd/api-server/Dockerfile"
	gitRepository: "https://github.com/coffeenights/conure.git"
	imagePullSecretsName: "regcred"
	ociRepository: "registry-service.conure-system.svc.cluster.local:5000/services/backend-service"
	ociTag: "latest"
	nameSuffix: "test"
}
