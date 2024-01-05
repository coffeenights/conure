package config

type Config struct {
	DaprGrpcPort string `env:"API_DAPR_GRPC_PORT"`
	MongoDBURI   string `env:"API_MONGODB_URI"`
	MongoDBName  string `env:"API_MONGODB_NAME"`
}
