package models

import (
	apiConfig "github.com/coffeenights/conure/cmd/api-server/config"
	"github.com/coffeenights/conure/cmd/api-server/database"
	"github.com/coffeenights/conure/internal/config"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func SetupDB() (*database.MongoDB, error) {
	appConfig := config.LoadConfig(apiConfig.Config{})
	testDBName := appConfig.MongoDBName + "-test"
	client, err := database.ConnectToMongoDB(appConfig.MongoDBURI, testDBName)
	if err != nil {
		return nil, err
	}
	return &database.MongoDB{Client: client.Client, DBName: testDBName}, nil
}

func ComponentTemplate(appID primitive.ObjectID, name string) *Component {
	return &Component{
		ApplicationID: appID,
		Name:          name,
		Type:          "service",
		Settings: ComponentSettings{
			ResourcesSettings: ResourcesSettings{
				Replicas: 1,
				CPU:      0.5,
				Memory:   200,
			},
			SourceSettings: SourceSettings{
				Repository: "coffeenights/django:latest",
				Command:    "python manage.py runserver 0.0.0.0:8000",
			},
			NetworkSettings: NetworkSettings{
				Exposed: true,
				Type:    "public",
				Ports: []PortSettings{
					{
						HostPort:   8000,
						TargetPort: 8000,
						Protocol:   "tcp",
					},
				},
			},
			StorageSettings: []StorageSettings{
				{
					Size:      20,
					Name:      "Volume1",
					MountPath: "/tmp",
				},
			},
		},
	}
}
