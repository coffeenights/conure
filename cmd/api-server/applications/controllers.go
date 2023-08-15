package applications

import (
	"github.com/coffeenights/conure/pkg/client/oam_conure"
	"github.com/gin-gonic/gin"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"log"
	"net/http"
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
		r := ApplicationResponse{
			Name:          app.ObjectMeta.Name,
			Description:   app.ObjectMeta.Namespace,
			EnvironmentId: "",
			AccountId:     0,
		}
		response = append(response, r)
	}
	c.JSON(http.StatusOK, response)
}
