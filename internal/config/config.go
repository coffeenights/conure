package config

import (
	"log"
	"os"
	"reflect"
	"strconv"
)

func LoadConfig[C any](config C) *C {
	v := reflect.ValueOf(&config).Elem()
	t := reflect.TypeOf(&config).Elem()

	for i := 0; i < v.NumField(); i++ {
		field := t.Field(i)
		envName, ok := field.Tag.Lookup("env")
		if !ok {
			continue // Skip fields without an 'env' tag
		}

		env, exist := os.LookupEnv(envName)
		if !exist {
			log.Fatalf("environment variable not found: %s", envName)
		}

		// Handle different field types appropriately
		switch v.Field(i).Kind() {
		case reflect.String:
			v.Field(i).SetString(env)
		case reflect.Int, reflect.Int64:
			intValue, err := strconv.ParseInt(env, 10, 64)
			if err != nil {
				log.Fatalf("failed to parse int64 for %s: %v", envName, err)
			}
			v.Field(i).SetInt(intValue)
		case reflect.Bool:
			boolValue, err := strconv.ParseBool(env)
			if err != nil {
				log.Fatalf("failed to parse bool for %s: %v", envName, err)
			}
			v.Field(i).SetBool(boolValue)
		default:
			// Optionally, handle unsupported field types or return an error
			log.Fatalf("unsupported field type %s for field %s", v.Field(i).Type(), field.Name)
		}
	}

	return &config
}
