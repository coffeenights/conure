/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
// Code generated by client-gen. DO NOT EDIT.

package v1alpha1

import (
	"net/http"

	v1alpha1 "github.com/coffeenights/conure/apis/core/v1alpha1"
	"github.com/coffeenights/conure/pkg/client/core_conure/scheme"
	rest "k8s.io/client-go/rest"
)

type CoreV1alpha1Interface interface {
	RESTClient() rest.Interface
	ActionDefinitionsGetter
	ApplicationsGetter
	WorkflowsGetter
	WorkflowRunsGetter
}

// CoreV1alpha1Client is used to interact with features provided by the  group.
type CoreV1alpha1Client struct {
	restClient rest.Interface
}

func (c *CoreV1alpha1Client) ActionDefinitions(namespace string) ActionDefinitionInterface {
	return newActionDefinitions(c, namespace)
}

func (c *CoreV1alpha1Client) Applications(namespace string) ApplicationInterface {
	return newApplications(c, namespace)
}

func (c *CoreV1alpha1Client) Workflows(namespace string) WorkflowInterface {
	return newWorkflows(c, namespace)
}

func (c *CoreV1alpha1Client) WorkflowRuns(namespace string) WorkflowRunInterface {
	return newWorkflowRuns(c, namespace)
}

// NewForConfig creates a new CoreV1alpha1Client for the given config.
// NewForConfig is equivalent to NewForConfigAndClient(c, httpClient),
// where httpClient was generated with rest.HTTPClientFor(c).
func NewForConfig(c *rest.Config) (*CoreV1alpha1Client, error) {
	config := *c
	if err := setConfigDefaults(&config); err != nil {
		return nil, err
	}
	httpClient, err := rest.HTTPClientFor(&config)
	if err != nil {
		return nil, err
	}
	return NewForConfigAndClient(&config, httpClient)
}

// NewForConfigAndClient creates a new CoreV1alpha1Client for the given config and http client.
// Note the http client provided takes precedence over the configured transport values.
func NewForConfigAndClient(c *rest.Config, h *http.Client) (*CoreV1alpha1Client, error) {
	config := *c
	if err := setConfigDefaults(&config); err != nil {
		return nil, err
	}
	client, err := rest.RESTClientForConfigAndClient(&config, h)
	if err != nil {
		return nil, err
	}
	return &CoreV1alpha1Client{client}, nil
}

// NewForConfigOrDie creates a new CoreV1alpha1Client for the given config and
// panics if there is an error in the config.
func NewForConfigOrDie(c *rest.Config) *CoreV1alpha1Client {
	client, err := NewForConfig(c)
	if err != nil {
		panic(err)
	}
	return client
}

// New creates a new CoreV1alpha1Client for the given RESTClient.
func New(c rest.Interface) *CoreV1alpha1Client {
	return &CoreV1alpha1Client{c}
}

func setConfigDefaults(config *rest.Config) error {
	gv := v1alpha1.SchemeGroupVersion
	config.GroupVersion = &gv
	config.APIPath = "/apis"
	config.NegotiatedSerializer = scheme.Codecs.WithoutConversion()

	if config.UserAgent == "" {
		config.UserAgent = rest.DefaultKubernetesUserAgent()
	}

	return nil
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *CoreV1alpha1Client) RESTClient() rest.Interface {
	if c == nil {
		return nil
	}
	return c.restClient
}
