package controllers

import (
	"context"
	"fmt"
	api_config "github.com/coffeenights/conure/cmd/api-server/config"
	apps_pb "github.com/coffeenights/conure/cmd/api-server/protos/apps"
	"github.com/coffeenights/conure/internal/config"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"log"
	"net/http"
	"time"
)

func ListApplications(c *gin.Context) {
	apiConfig := config.LoadConfig(api_config.Config{})
	log.Println("Dialing ...")
	conn, err := grpc.Dial(fmt.Sprintf("localhost:%s", apiConfig.DaprGrpcPort), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"Error": err})
	}

	applicationClient := apps_pb.NewApplicationServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*2)
	defer cancel()
	ctx = metadata.AppendToOutgoingContext(ctx, "dapr-app-id", "services-apps-api")
	response, err := applicationClient.ListApplications(ctx, &apps_pb.ListApplicationsRequest{
		AccountId: 0,
	})
	if err != nil {
		log.Fatal(err)
	}
	c.JSON(http.StatusOK, response)
	defer conn.Close()
}
