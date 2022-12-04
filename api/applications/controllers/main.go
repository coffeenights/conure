package controllers

import (
	"context"
	apps_pb "github.com/coffeenights/conure/api/protos/apps"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"log"
	"net/http"
	"time"
)

func ListApplications(c *gin.Context) {
	conn, err := grpc.Dial("localhost:50007", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatal(err)
	}

	applicationClient := apps_pb.NewApplicationServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*2)
	defer cancel()
	ctx = metadata.AppendToOutgoingContext(ctx, "dapr-app-id", "conure_apps_api")
	response, err := applicationClient.ListApplications(ctx, &apps_pb.ListApplicationsRequest{
		AccountId: 0,
	})
	if err != nil {
		log.Fatal(err)
	}
	c.JSON(http.StatusOK, response)
	defer conn.Close()
}
