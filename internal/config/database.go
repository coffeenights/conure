package config

import (
	"fmt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"log"
	"net"
	"net/url"
	"strings"
)

func GetDbDSN(dbUrl string) string {
	// Parse the DB URL
	uri, err := url.Parse(dbUrl)
	if err != nil {
		panic(err)
	}
	dbName := strings.TrimLeft(uri.Path, "/")
	host, dbPort, _ := net.SplitHostPort(uri.Host)
	password, _ := uri.User.Password()
	dsn := fmt.Sprintf("host=%s user=%s password=%s database=%s port=%s", host, uri.User.Username(), password, dbName, dbPort)
	return dsn
}

func GetDbConnection(dsn string) *gorm.DB {
	log.Println("Connecting to the database ...")
	conn, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to the database: %s", err)
	}
	return conn
}
