package applications

import (
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"log"
	"net/http"
)

func (a *ApiHandler) ListApplications(c *gin.Context) {
	// Escape the organizationID
	if _, err := primitive.ObjectIDFromHex(c.Param("organizationID")); err != nil {
		log.Printf("Error parsing organizationID: %v\n", err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	org := Organization{}
	_, err := org.GetById(a.MongoDB, c.Param("organizationID"))
	if err != nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	handlers, err := ListOrganizationApplications(c.Param("organizationID"), a.MongoDB)
	if err != nil {
		log.Printf("Error getting applications list: %v\n", err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	response := ApplicationListResponse{}
	response.Organization.ParseModelToResponse(&org)
	applicationResponses := make([]ApplicationResponse, len(handlers))
	for i, handler := range handlers {
		r := ApplicationResponse{
			Application: handler.Model,
		}
		applicationResponses[i] = r
	}
	response.Applications = applicationResponses
	c.JSON(http.StatusOK, response)
	return
}

func (a *ApiHandler) DetailApplication(c *gin.Context) {
	// Escape the organizationID
	if _, err := primitive.ObjectIDFromHex(c.Param("organizationID")); err != nil {
		log.Printf("Error parsing organizationID: %v\n", err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	// Escape the applicationID
	if _, err := primitive.ObjectIDFromHex(c.Param("applicationID")); err != nil {
		log.Printf("Error parsing applicationID: %v\n", err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	handler, err := NewApplicationHandler(a.MongoDB)
	err = handler.GetApplicationByID(c.Param("applicationID"))
	if err != nil {
		log.Printf("Error getting application: %v\n", err)
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	response := ApplicationResponse{
		Application: handler.Model,
	}
	c.JSON(http.StatusOK, response)
	return
}
