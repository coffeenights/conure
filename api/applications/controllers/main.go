package controllers

import (
	"context"
	apps_pb "github.com/coffeenights/conure/api/protos/apps"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"log"
	"net/http"
)

func ListApplications(c *gin.Context) {
	conn, err := grpc.Dial("localhost:50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatal(err)
	}

	applicationClient := apps_pb.NewApplicationServiceClient(conn)
	response, err := applicationClient.ListApplications(context.Background(), &apps_pb.ListApplicationsRequest{
		AccountId: 0,
	})
	if err != nil {
		log.Fatal(err)
	}
	c.JSON(http.StatusOK, response)
	defer conn.Close()
}
