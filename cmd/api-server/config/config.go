package config

type Config struct {
	DaprGrpcPort       string `env:"API_DAPR_GRPC_PORT"`
	MongoDBURI         string `env:"API_MONGODB_URI"`
	MongoDBName        string `env:"API_MONGODB_NAME"`
	JWTSecret          string `env:"JWT_SECRET"`
	JWTExpiration      int    `env:"JWT_EXPIRATION_DAYS"`
	ProviderSource     string `env:"PROVIDER_SOURCE"`
	AESStorageStrategy string `env:"AES_STORAGE_STRATEGY"`
	AuthServiceURL     string `env:"AUTH_SERVICE_URL"`
	AuthStrategySystem string `env:"AUTH_STRATEGY_SYSTEM"`
	FrontendDomain     string `env:"FRONTEND_DOMAIN"`
	CookieSecure       bool   `env:"COOKIE_SECURE"`
	CorsOrigins        string `env:"CORS_ORIGINS"`
}
