# Conure

## Running the development environment

### 1. Pre requisites

- Install docker and docker-compose.
- Install the Dapr framework, the CLI and initialize the local environment (https://docs.dapr.io/getting-started/install-dapr-cli/)
- You will need a postgres and a redis server available. You could use the ones present in the docker-compose.yml file, simply run:
```shell
$ docker-compose run -d db
$ docker-compose run -d redis 
```
- Set the environments file by making a copy of the `config.env` file and renaming it to `.env`. 

> **_NOTE:_** Take in consideration that the `DB_URL` variable inside the config.env file is set with the credentials of the postgres in the docker-compose file, if you want to use your own postgres instance you must change the content of the `DB_URL` to fit your instance.

### 2. Run the dapr sidecars

from the project's root folder, you must run the following command:
```shell
$ dapr run -f ./dapr/dev/.
```

This will run the sidecars for the applications, and now you can run each application individually, i.e.
```shell
$ go run ./cmd/api-server/main.go
```