package applications

import (
	"github.com/coffeenights/conure/cmd/api-server/middlewares"
	"github.com/gin-gonic/gin"
)

func GenerateRoutes(relativePath string, r *gin.Engine, appHandler *ApiHandler) {
	applications := r.Group(relativePath, middlewares.CheckAuthenticatedUser(appHandler.Config, appHandler.MongoDB))
	{
		// Org / app identity ---------------------------------------------------
		applications.GET("/", appHandler.ListOrganization)
		applications.POST("/", appHandler.CreateOrganization)
		applications.GET("/:organizationID", appHandler.DetailOrganization)
		applications.GET("/:organizationID/a", appHandler.ListApplications)
		applications.POST("/:organizationID/a", appHandler.CreateApplication)
		applications.GET("/:organizationID/a/:applicationID", appHandler.DetailApplication)

		// Environments ---------------------------------------------------------
		applications.POST("/:organizationID/a/:applicationID/e", appHandler.CreateEnvironment)
		applications.DELETE("/:organizationID/a/:applicationID/e/:environment", appHandler.DeleteEnvironment)

		// Bulk env deploy: replaces the legacy `PUT /e/:env` DeployApplication. RPC
		// verb mirrors per-component deploy.
		applications.POST("/:organizationID/a/:applicationID/e/:environment/deploy", appHandler.DeployEnvDrafts)

		// App-wide component identity -----------------------------------------
		applications.GET("/:organizationID/a/:applicationID/c", appHandler.ListComponents)
		applications.POST("/:organizationID/a/:applicationID/c", appHandler.CreateComponent)
		applications.GET("/:organizationID/a/:applicationID/c/:componentID", appHandler.GetComponent)
		applications.DELETE("/:organizationID/a/:applicationID/c/:componentID", appHandler.DeleteComponent)
		applications.POST("/:organizationID/a/:applicationID/c/:componentID/promote", appHandler.PromoteComponent)

		// Env-scoped reads + revisions ----------------------------------------
		applications.GET("/:organizationID/a/:applicationID/e/:environment/c", appHandler.ListComponentsInEnv)
		applications.GET("/:organizationID/a/:applicationID/e/:environment/c/:componentID", appHandler.GetComponentInEnv)
		applications.POST("/:organizationID/a/:applicationID/e/:environment/c/:componentID/revisions", appHandler.CreateRevision)
		applications.GET("/:organizationID/a/:applicationID/e/:environment/c/:componentID/revisions", appHandler.ListRevisions)
		applications.PUT("/:organizationID/a/:applicationID/e/:environment/c/:componentID/revisions/:revID", appHandler.UpdateDraftRevision)
		applications.POST("/:organizationID/a/:applicationID/e/:environment/c/:componentID/deploy", appHandler.DeployRevision)
		applications.POST("/:organizationID/a/:applicationID/e/:environment/c/:componentID/revisions/:revID/deploy", appHandler.DeployRevisionByID)
		applications.POST("/:organizationID/a/:applicationID/e/:environment/c/:componentID/uninstall", appHandler.UninstallFromEnv)
	}
}
