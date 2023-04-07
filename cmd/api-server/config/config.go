package config

type Config struct {
	DaprGrpcPort string `env:"API_DAPR_GRPC_PORT"`
}
