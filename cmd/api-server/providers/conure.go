package providers

import (
	"context"
	"fmt"
	"log"

	conurev1alpha1 "github.com/coffeenights/conure/apis/core/v1alpha1"
	k8sUtils "github.com/coffeenights/conure/internal/k8s"
	corev1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ProviderDispatcherConure is the per-environment driver for applying and
// removing Component CRDs. The plural name is historical; with Vela gone it
// is the only provider, called directly from handlers.
type ProviderDispatcherConure struct {
	OrganizationID  string
	ApplicationID   string
	ApplicationName string
	Namespace       string
	Environment     string
}

// EnsureNamespace creates the env namespace if missing. Idempotent — an
// existing namespace is reused without error.
func (p *ProviderDispatcherConure) EnsureNamespace(ctx context.Context, clientset *k8sUtils.GenericClientset) error {
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
	_, err := clientset.K8s.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
	if k8sErrors.IsAlreadyExists(err) {
		return nil
	}
	return err
}

// EnsureApplicationCRD creates the Application CRD in the env namespace if
// missing. The Application CRD is scoped per-environment in this model.
func (p *ProviderDispatcherConure) EnsureApplicationCRD(ctx context.Context, clientset *k8sUtils.GenericClientset, app *conurev1alpha1.Application) error {
	apps := clientset.Conure.CoreV1alpha1().Applications(p.Namespace)
	_, err := apps.Create(ctx, app, metav1.CreateOptions{})
	if k8sErrors.IsAlreadyExists(err) {
		return nil
	}
	return err
}

// SyncComponentVariables creates or updates the ConfigMap (plain variables)
// and Secret (encrypted secrets) backing a component's runtime values.
func (p *ProviderDispatcherConure) SyncComponentVariables(clientset *k8sUtils.GenericClientset, cv ComponentVariables) error {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-variables", cv.ComponentName),
			Namespace: p.Namespace,
		},
		Data: cv.Variables,
	}
	if err := k8sUtils.CreateOrUpdateConfigMap(clientset, p.Namespace, cm); err != nil {
		return fmt.Errorf("syncing configmap for component %q: %w", cv.ComponentName, err)
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-secrets", cv.ComponentName),
			Namespace: p.Namespace,
		},
		Type:       corev1.SecretTypeOpaque,
		StringData: cv.Secrets,
	}
	if err := k8sUtils.CreateOrUpdateSecret(clientset, p.Namespace, secret); err != nil {
		return fmt.Errorf("syncing secret for component %q: %w", cv.ComponentName, err)
	}
	return nil
}

// ApplyComponent creates or updates a single Component CRD in the env
// namespace, syncing its variables ConfigMap/Secret first. Used by the per-
// component deploy path.
func (p *ProviderDispatcherConure) ApplyComponent(ctx context.Context, app *conurev1alpha1.Application, component *conurev1alpha1.Component, cv ComponentVariables) error {
	clientset, err := k8sUtils.GetClientset()
	if err != nil {
		return err
	}

	if err = p.EnsureNamespace(ctx, clientset); err != nil {
		return err
	}
	if err = p.EnsureApplicationCRD(ctx, clientset, app); err != nil {
		return err
	}
	if err = p.SyncComponentVariables(clientset, cv); err != nil {
		return err
	}

	components := clientset.Conure.CoreV1alpha1().Components(p.Namespace)
	existing, err := components.Get(ctx, component.Name, metav1.GetOptions{})
	if k8sErrors.IsNotFound(err) {
		_, err = components.Create(ctx, component, metav1.CreateOptions{})
		if err != nil {
			return err
		}
		log.Printf("Created component %q in %s\n", component.Name, p.Namespace)
		return nil
	}
	if err != nil {
		return err
	}
	component.ResourceVersion = existing.ResourceVersion
	_, err = components.Update(ctx, component, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	log.Printf("Updated component %q in %s\n", component.Name, p.Namespace)
	return nil
}

// DeleteComponentCRD removes the Component CRD in the env namespace, plus
// its variables ConfigMap and Secret. Idempotent — missing resources do not
// produce errors.
func (p *ProviderDispatcherConure) DeleteComponentCRD(ctx context.Context, componentName string) error {
	clientset, err := k8sUtils.GetClientset()
	if err != nil {
		return err
	}

	err = clientset.Conure.CoreV1alpha1().Components(p.Namespace).Delete(ctx, componentName, metav1.DeleteOptions{})
	if err != nil && !k8sErrors.IsNotFound(err) {
		return err
	}

	cmName := fmt.Sprintf("%s-variables", componentName)
	if err := clientset.K8s.CoreV1().ConfigMaps(p.Namespace).Delete(ctx, cmName, metav1.DeleteOptions{}); err != nil && !k8sErrors.IsNotFound(err) {
		return err
	}
	secretName := fmt.Sprintf("%s-secrets", componentName)
	if err := clientset.K8s.CoreV1().Secrets(p.Namespace).Delete(ctx, secretName, metav1.DeleteOptions{}); err != nil && !k8sErrors.IsNotFound(err) {
		return err
	}
	return nil
}
