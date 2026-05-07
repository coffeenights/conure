package models

import (
	apiConfig "github.com/coffeenights/conure/cmd/api-server/config"
	"github.com/coffeenights/conure/cmd/api-server/database"
	"github.com/coffeenights/conure/internal/config"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func SetupDB() (*database.MongoDB, error) {
	appConfig := config.LoadConfig(apiConfig.Config{})
	testDBName := appConfig.MongoDBName + "-test-models"
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
		Values: map[string]interface{}{
			"resources": map[string]interface{}{
				"replicas": 1,
				"cpu":      "500m",
				"memory":   "200Mi",
			},
			"source": map[string]interface{}{
				"ociRepository": "coffeenights/django:latest",
				"command":       []string{"python", "manage.py", "runserver", "0.0.0.0:8000"},
			},
			"network": map[string]interface{}{
				"exposed": true,
				"type":    "public",
				"ports": []map[string]interface{}{
					{
						"hostPort":   8000,
						"targetPort": 8000,
						"protocol":   "tcp",
					},
				},
			},
			"storage": []map[string]interface{}{
				{
					"size":      "20Gi",
					"name":      "Volume1",
					"mountPath": "/tmp",
				},
			},
		},
	}
}
