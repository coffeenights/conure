package config

import (
	"log"
	"os"
	"reflect"
)

func LoadConfig[C any](config C) *C {
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
