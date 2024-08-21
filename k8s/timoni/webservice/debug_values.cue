@if(debug)

package main

// Values used by debug_tool.cue.
// Debug example 'cue cmd -t debug -t name=test -t namespace=test -t mv=1.0.0 -t kv=1.28.0 build'.

"values": {
  "resources": {
    "replicas": "2",
    "cpu": "200m",
    "memory": "256Mi"
  },
  "network": {
    "exposed": true,
    "type": "public",
    "ports": [
      {
        "hostPort": "9091",
        "targetPort": "9091",
        "protocol": "TCP"
      }
    ]
  },
  "source": {
    "sourceType": "git",
    "gitRepository": "https://github.com/mredvard/fastapi_demo.git",
    "gitBranch": "main",
    "buildTool": "dockerfile",
    "dockerfilePath": "Dockerfile",
    "tag": "latest",
    "command": ["uvicorn", "main:app", "--host", "0.0.0.0", "--port", "8000"],
    "workingDir": "/app",
    "imagePullSecretsName": "regcred"
  },
  "storage": [
    {
      "size": "2Gi",
      "name": "temporal",
      "mountPath": "/mnt/storage"
    },
    {
      "size": "3Gi",
      "name": "cache",
      "mountPath": "/mnt/storage2"
    }
  ]
}