package applications

import (
	"github.com/coffeenights/conure/pkg/client/oam_conure"
	"github.com/gin-gonic/gin"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"log"
	"net/http"
	"strings"
)

func getClientset() (*oam_conure.Clientset, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		kubeconfig :=
			clientcmd.NewDefaultClientConfigLoadingRules().GetDefaultFilename()
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return nil, err
		}
	}
	return oam_conure.NewForConfig(config)
}

func ListApplications(c *gin.Context) {
	// apiConfig := config.LoadConfig(api_config.Config{})
	log.Println("Dialing ...")

	// q is the query param that represents the search term
	q := c.DefaultQuery("q", "")

	// creates the clientset
	clientset, err := getClientset()
	if err != nil {
		log.Fatal(err.Error())
	}
	applications, err := clientset.OamV1alpha1().Applications("default").List(c, metav1.ListOptions{})
	if err != nil {
		log.Fatal(err.Error())
	}

	var response []ApplicationResponse
	for _, app := range applications.Items {

		// Apply filtering based on the query parameter
		if q != "" && !strings.Contains(app.ObjectMeta.Name, q) {
			continue
		}

		var r ApplicationResponse
		r.FromClientsetToResponse(&app)
		response = append(response, r)
	}
	c.JSON(http.StatusOK, response)
}

func DetailApplications(c *gin.Context) {
	// apiConfig := config.LoadConfig(api_config.Config{})
	log.Println("Dialing ...")

	// q is the query param that represents the search term
	applicationName := c.Param("applicationName")

	// creates the clientset
	clientset, err := getClientset()
	if err != nil {
		log.Fatal(err.Error())
	}
	application, err := clientset.OamV1alpha1().Applications("default").Get(c, applicationName, metav1.GetOptions{})
	if err != nil {
		log.Fatal(err.Error())
	}

	var response ApplicationResponse
	response.FromClientsetToResponse(application)
	c.JSON(http.StatusOK, response)
}
