package config

import (
	"fmt"
	"log"
	"net"
	"net/url"
	"os"
	"reflect"
	"strings"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type Config struct {
	DbUrl     string `env:"DB_URL"`
	SecretKey string `env:"SECRET_KEY"`
}

func (c *Config) GetDbDSN() string {
	// Parse the DB URL
	uri, err := url.Parse(c.DbUrl)
	if err != nil {
		panic(err)
	}
	dbName := strings.TrimLeft(uri.Path, "/")
	host, dbPort, _ := net.SplitHostPort(uri.Host)
	password, _ := uri.User.Password()
	dsn := fmt.Sprintf("host=%s user=%s password=%s database=%s port=%s", host, uri.User.Username(), password, dbName, dbPort)
	return dsn
}

func LoadConfig() *Config {
	config := Config{}
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

func GetDbConnection(dsn string) *gorm.DB {
	log.Println("Connecting to the database ...")
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to the database: %s", err)
	}
	return db
}
