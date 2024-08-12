@if(debug)

package main

// Values used by debug_tool.cue.
// Debug example 'cue cmd -t debug -t name=test -t namespace=test -t mv=1.0.0 -t kv=1.28.0 build'.
values: {
  "resources": {
    "replicas": 1,
    "cpu": "200m",
    "memory": "256Mi"
  },
  "sourceSettings": {
    "repository": "coffeenights/django:latest",
    "command": ["python", "manage.py", "runserver", "0.0.0.0:9091"],
    "workingDir": "/app"
  },
  "network": {
    "type": "public",
    "ports": [
      {
        "hostPort": 9091,
        "containerPort": 9091,
        "protocol": "TCP"
      }
    ]
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
