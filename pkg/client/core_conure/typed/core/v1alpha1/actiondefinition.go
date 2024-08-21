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
	"context"
	"time"

	v1alpha1 "github.com/coffeenights/conure/apis/core/v1alpha1"
	scheme "github.com/coffeenights/conure/pkg/client/core_conure/scheme"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
)

// ActionDefinitionsGetter has a method to return a ActionDefinitionInterface.
// A group's client should implement this interface.
type ActionDefinitionsGetter interface {
	ActionDefinitions(namespace string) ActionDefinitionInterface
}

// ActionDefinitionInterface has methods to work with ActionDefinition resources.
type ActionDefinitionInterface interface {
	Create(ctx context.Context, actionDefinition *v1alpha1.ActionDefinition, opts v1.CreateOptions) (*v1alpha1.ActionDefinition, error)
	Update(ctx context.Context, actionDefinition *v1alpha1.ActionDefinition, opts v1.UpdateOptions) (*v1alpha1.ActionDefinition, error)
	UpdateStatus(ctx context.Context, actionDefinition *v1alpha1.ActionDefinition, opts v1.UpdateOptions) (*v1alpha1.ActionDefinition, error)
	Delete(ctx context.Context, name string, opts v1.DeleteOptions) error
	DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error
	Get(ctx context.Context, name string, opts v1.GetOptions) (*v1alpha1.ActionDefinition, error)
	List(ctx context.Context, opts v1.ListOptions) (*v1alpha1.ActionDefinitionList, error)
	Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error)
	Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha1.ActionDefinition, err error)
	ActionDefinitionExpansion
}

// actionDefinitions implements ActionDefinitionInterface
type actionDefinitions struct {
	client rest.Interface
	ns     string
}

// newActionDefinitions returns a ActionDefinitions
func newActionDefinitions(c *CoreV1alpha1Client, namespace string) *actionDefinitions {
	return &actionDefinitions{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Get takes name of the actionDefinition, and returns the corresponding actionDefinition object, and an error if there is any.
func (c *actionDefinitions) Get(ctx context.Context, name string, options v1.GetOptions) (result *v1alpha1.ActionDefinition, err error) {
	result = &v1alpha1.ActionDefinition{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("actiondefinitions").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do(ctx).
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of ActionDefinitions that match those selectors.
func (c *actionDefinitions) List(ctx context.Context, opts v1.ListOptions) (result *v1alpha1.ActionDefinitionList, err error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	result = &v1alpha1.ActionDefinitionList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("actiondefinitions").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Do(ctx).
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested actionDefinitions.
func (c *actionDefinitions) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	opts.Watch = true
	return c.client.Get().
		Namespace(c.ns).
		Resource("actiondefinitions").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Watch(ctx)
}

// Create takes the representation of a actionDefinition and creates it.  Returns the server's representation of the actionDefinition, and an error, if there is any.
func (c *actionDefinitions) Create(ctx context.Context, actionDefinition *v1alpha1.ActionDefinition, opts v1.CreateOptions) (result *v1alpha1.ActionDefinition, err error) {
	result = &v1alpha1.ActionDefinition{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("actiondefinitions").
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(actionDefinition).
		Do(ctx).
		Into(result)
	return
}

// Update takes the representation of a actionDefinition and updates it. Returns the server's representation of the actionDefinition, and an error, if there is any.
func (c *actionDefinitions) Update(ctx context.Context, actionDefinition *v1alpha1.ActionDefinition, opts v1.UpdateOptions) (result *v1alpha1.ActionDefinition, err error) {
	result = &v1alpha1.ActionDefinition{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("actiondefinitions").
		Name(actionDefinition.Name).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(actionDefinition).
		Do(ctx).
		Into(result)
	return
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *actionDefinitions) UpdateStatus(ctx context.Context, actionDefinition *v1alpha1.ActionDefinition, opts v1.UpdateOptions) (result *v1alpha1.ActionDefinition, err error) {
	result = &v1alpha1.ActionDefinition{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("actiondefinitions").
		Name(actionDefinition.Name).
		SubResource("status").
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(actionDefinition).
		Do(ctx).
		Into(result)
	return
}

// Delete takes name of the actionDefinition and deletes it. Returns an error if one occurs.
func (c *actionDefinitions) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("actiondefinitions").
		Name(name).
		Body(&opts).
		Do(ctx).
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *actionDefinitions) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	var timeout time.Duration
	if listOpts.TimeoutSeconds != nil {
		timeout = time.Duration(*listOpts.TimeoutSeconds) * time.Second
	}
	return c.client.Delete().
		Namespace(c.ns).
		Resource("actiondefinitions").
		VersionedParams(&listOpts, scheme.ParameterCodec).
		Timeout(timeout).
		Body(&opts).
		Do(ctx).
		Error()
}

// Patch applies the patch and returns the patched actionDefinition.
func (c *actionDefinitions) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha1.ActionDefinition, err error) {
	result = &v1alpha1.ActionDefinition{}
	err = c.client.Patch(pt).
		Namespace(c.ns).
		Resource("actiondefinitions").
		Name(name).
		SubResource(subresources...).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(data).
		Do(ctx).
		Into(result)
	return
}