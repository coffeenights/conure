package applications

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	coreOAMDevClientset "github.com/oam-dev/kubevela-core-api/pkg/generated/client/clientset/versioned"
	k8sV1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/coffeenights/conure/pkg/client/oam_conure"
)

type genericClientset struct {
	Conure *oam_conure.Clientset
	K8s    *kubernetes.Clientset
	Vela   *coreOAMDevClientset.Clientset
}

func getClientset() (*genericClientset, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		kubeconfig :=
			clientcmd.NewDefaultClientConfigLoadingRules().GetDefaultFilename()
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return nil, err
		}
	}
	k8s, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	vela, err := coreOAMDevClientset.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	conure, err := oam_conure.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	return &genericClientset{Conure: conure, K8s: k8s, Vela: vela}, nil
}

func ListApplications(c *gin.Context) {
	// q is the query param that represents the search term
	q := c.DefaultQuery("q", "")

	// creates the clientset
	clientset, err := getClientset()
	if err != nil {
		log.Fatal(err.Error())
	}

	applications, err := clientset.Vela.CoreV1beta1().Applications("default").List(c, metav1.ListOptions{})
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
		r.FromVelaClientsetToResponse(&app)

		if err != nil {
			log.Fatal(err.Error())
		}

		labels := map[string]string{
			"app.oam.dev/name": app.Name,
		}

		components, err := getDeploymentByLabels(clientset.K8s, "default", labels)
		if err != nil {
			fmt.Printf("Error getting deployment: %v\n", err)
		}

		for _, deployment := range components {
			var c ServiceComponentResponse
			services, err := getServicesByLabels(clientset.K8s, "default", labels)
			if err != nil {
				fmt.Printf("Error getting services: %v\n", err)
			}
			c.FromClientsetToResponse(deployment, services)
			r.Components = append(r.Components, c)
		}
		r.TotalComponents = len(r.Components)

		response = append(response, r)
	}
	c.JSON(http.StatusOK, response)
}

func DetailApplications(c *gin.Context) {
	// q is the query param that represents the search term
	applicationName := c.Param("applicationName")

	// creates the clientset
	clientset, err := getClientset()
	if err != nil {
		log.Fatal(err.Error())
	}

	application, err := clientset.Vela.CoreV1beta1().Applications("default").Get(c, applicationName, metav1.GetOptions{})
	if err != nil {
		log.Fatal(err.Error())
	}

	var response ApplicationResponse
	response.FromVelaClientsetToResponse(application)

	labels := map[string]string{
		"app.oam.dev/name": application.Name,
	}
	deployments, err := getDeploymentByLabels(clientset.K8s, "default", labels)
	if err != nil {
		fmt.Printf("Error getting deployment: %v\n", err)
	}

	for _, deployment := range deployments {
		services, err := getServicesByLabels(clientset.K8s, "default", labels)
		if err != nil {
			fmt.Printf("Error getting services: %v\n", err)
		}

		var c ServiceComponentResponse
		c.FromClientsetToResponse(deployment, services)
		response.Components = append(response.Components, c)
	}
	response.TotalComponents = len(response.Components)

	c.JSON(http.StatusOK, response)
}

func getDeploymentByLabels(clientset *kubernetes.Clientset, namespace string, labels map[string]string) ([]k8sV1.Deployment, error) {
	deploymentsClient := clientset.AppsV1().Deployments(namespace)
	var labelSelector []string
	for key, value := range labels {
		labelSelector = append(labelSelector, fmt.Sprintf("%s=%s", key, value))
	}

	listOptions := metav1.ListOptions{
		LabelSelector: strings.Join(labelSelector, ","),
	}

	deployments, err := deploymentsClient.List(context.TODO(), listOptions)
	if err != nil {
		return nil, err
	}

	if len(deployments.Items) == 0 {
		return nil, fmt.Errorf("no deployment found with label selector: %s", labelSelector)
	}

	return deployments.Items, nil
}

func getServicesByLabels(clientset *kubernetes.Clientset, namespace string, labels map[string]string) ([]corev1.Service, error) {
	servicesClient := clientset.CoreV1().Services(namespace)

	var labelSelector []string
	for key, value := range labels {
		labelSelector = append(labelSelector, fmt.Sprintf("%s=%s", key, value))
	}

	listOptions := metav1.ListOptions{
		LabelSelector: strings.Join(labelSelector, ","),
	}

	services, err := servicesClient.List(context.TODO(), listOptions)
	if err != nil {
		return nil, err
	}

	return services.Items, nil
}
