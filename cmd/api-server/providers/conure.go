package providers

import (
	"context"
	"log"

	conurev1alpha1 "github.com/coffeenights/conure/apis/core/v1alpha1"
	"github.com/coffeenights/conure/cmd/api-server/conureerrors"
	k8sUtils "github.com/coffeenights/conure/internal/k8s"
	corev1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ProviderDispatcherConure struct {
	OrganizationID  string
	ApplicationID   string
	ApplicationName string
	Namespace       string
	Environment     string
}

func (p *ProviderDispatcherConure) createNamespace(clientset *k8sUtils.GenericClientset) error {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: p.Namespace,
			Labels: map[string]string{
				k8sUtils.ApplicationIDLabel:  p.ApplicationID,
				k8sUtils.OrganizationIDLabel: p.OrganizationID,
				k8sUtils.EnvironmentLabel:    p.Environment,
			},
		},
	}
	_, err := clientset.K8s.CoreV1().Namespaces().Create(context.Background(), ns, metav1.CreateOptions{})
	if k8sErrors.IsAlreadyExists(err) {
		log.Printf("Namespace %s already exists, reusing it\n", p.Namespace)
		return nil
	}
	return err
}

func (p *ProviderDispatcherConure) DeployApplication(app *conurev1alpha1.Application, components []conurev1alpha1.Component) error {
	clientset, err := k8sUtils.GetClientset()
	if err != nil {
		log.Printf("Error getting clientset: %v\n", err)
		return err
	}

	if err = p.createNamespace(clientset); err != nil {
		return err
	}

	coreClient := clientset.Conure.CoreV1alpha1()

	_, err = coreClient.Applications(p.Namespace).Create(context.Background(), app, metav1.CreateOptions{})
	if k8sErrors.IsAlreadyExists(err) {
		log.Printf("Application %s already exists\n", p.ApplicationName)
		return conureerrors.ErrApplicationExists
	}
	if err != nil {
		return err
	}
	log.Printf("Created application %q\n", p.ApplicationName)

	for i := range components {
		_, err = coreClient.Components(p.Namespace).Create(context.Background(), &components[i], metav1.CreateOptions{})
		if err != nil {
			return err
		}
		log.Printf("Created component %q\n", components[i].Name)
	}

	return nil
}

func (p *ProviderDispatcherConure) UpdateApplication(app *conurev1alpha1.Application, components []conurev1alpha1.Component) error {
	clientset, err := k8sUtils.GetClientset()
	if err != nil {
		log.Printf("Error getting clientset: %v\n", err)
		return err
	}

	coreClient := clientset.Conure.CoreV1alpha1()

	existing, err := coreClient.Applications(p.Namespace).Get(context.Background(), p.ApplicationName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	app.ResourceVersion = existing.ResourceVersion
	_, err = coreClient.Applications(p.Namespace).Update(context.Background(), app, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	log.Printf("Updated application %q\n", p.ApplicationName)

	for i := range components {
		comp := &components[i]
		existingComp, err := coreClient.Components(p.Namespace).Get(context.Background(), comp.Name, metav1.GetOptions{})
		if k8sErrors.IsNotFound(err) {
			_, err = coreClient.Components(p.Namespace).Create(context.Background(), comp, metav1.CreateOptions{})
			if err != nil {
				return err
			}
			log.Printf("Created component %q\n", comp.Name)
			continue
		}
		if err != nil {
			return err
		}
		comp.ResourceVersion = existingComp.ResourceVersion
		_, err = coreClient.Components(p.Namespace).Update(context.Background(), comp, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
		log.Printf("Updated component %q\n", comp.Name)
	}

	return nil
}
