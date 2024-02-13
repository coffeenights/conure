package config

import (
	"log"
	"os"
	"reflect"

	apiConfig "github.com/coffeenights/conure/cmd/api-server/config"
)

func LoadConfig(config apiConfig.Config) *apiConfig.Config {
	v := reflect.ValueOf(&config).Elem()
	t := reflect.TypeOf(&config).Elem()
	for i := 0; i < v.NumField(); i++ {
		field := t.Field(i)
		envName, _ := field.Tag.Lookup("env")
		env, exist := os.LookupEnv(envName)
		if !exist {
			log.Fatalf("Environment variable not found: %s", envName)
		}
		v.Field(i).SetString(env)
	}
	return &config
}
