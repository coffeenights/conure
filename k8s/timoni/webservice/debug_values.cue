@if(debug)

package main

// Values used by debug_tool.cue.
// Debug example 'cue cmd -t debug -t name=test -t namespace=test -t mv=1.0.0 -t kv=1.28.0 build'.
values: {
  "resources": {
    "replicas": 1,
    "cpu": 0.2,
    "memory": 256
  },
  "source": {
    "repository": "coffeenights/django:latest",
    "command": "python manage.py runserver 0.0.0.0:9091"
    "workingDir": "/app"
  },
  "network": {
    "exposed": true,
    "type": "public",
    "ports": [
      {
        "host_port": 9091,
        "container_port": 9091,
        "protocol": "tcp"
      }
    ]
  },
  "storage": [
    {
      "size": 2,
      "name": "backend-service-pvc",
      "mount_path": "/mnt/storage"
    },
    {
      "size": 3,
      "name": "backend-service-2-pvc",
      "mount_path": "/mnt/storage2"
    }
  ]
}
