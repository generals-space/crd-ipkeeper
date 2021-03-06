/*
Copyright The Kubernetes Authors.

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

package v1

import (
	"time"

	v1 "github.com/generals-space/crd-ipkeeper/pkg/apis/ipkeeper/v1"
	scheme "github.com/generals-space/crd-ipkeeper/pkg/client/clientset/versioned/scheme"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
)

// StaticIPsGetter has a method to return a StaticIPInterface.
// A group's client should implement this interface.
type StaticIPsGetter interface {
	StaticIPs(namespace string) StaticIPInterface
}

// StaticIPInterface has methods to work with StaticIP resources.
type StaticIPInterface interface {
	Create(*v1.StaticIP) (*v1.StaticIP, error)
	Update(*v1.StaticIP) (*v1.StaticIP, error)
	UpdateStatus(*v1.StaticIP) (*v1.StaticIP, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteCollection(options *metav1.DeleteOptions, listOptions metav1.ListOptions) error
	Get(name string, options metav1.GetOptions) (*v1.StaticIP, error)
	List(opts metav1.ListOptions) (*v1.StaticIPList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.StaticIP, err error)
	StaticIPExpansion
}

// staticIPs implements StaticIPInterface
type staticIPs struct {
	client rest.Interface
	ns     string
}

// newStaticIPs returns a StaticIPs
func newStaticIPs(c *IpkeeperV1Client, namespace string) *staticIPs {
	return &staticIPs{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Get takes name of the staticIP, and returns the corresponding staticIP object, and an error if there is any.
func (c *staticIPs) Get(name string, options metav1.GetOptions) (result *v1.StaticIP, err error) {
	result = &v1.StaticIP{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("staticips").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of StaticIPs that match those selectors.
func (c *staticIPs) List(opts metav1.ListOptions) (result *v1.StaticIPList, err error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	result = &v1.StaticIPList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("staticips").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested staticIPs.
func (c *staticIPs) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	opts.Watch = true
	return c.client.Get().
		Namespace(c.ns).
		Resource("staticips").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Watch()
}

// Create takes the representation of a staticIP and creates it.  Returns the server's representation of the staticIP, and an error, if there is any.
func (c *staticIPs) Create(staticIP *v1.StaticIP) (result *v1.StaticIP, err error) {
	result = &v1.StaticIP{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("staticips").
		Body(staticIP).
		Do().
		Into(result)
	return
}

// Update takes the representation of a staticIP and updates it. Returns the server's representation of the staticIP, and an error, if there is any.
func (c *staticIPs) Update(staticIP *v1.StaticIP) (result *v1.StaticIP, err error) {
	result = &v1.StaticIP{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("staticips").
		Name(staticIP.Name).
		Body(staticIP).
		Do().
		Into(result)
	return
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().

func (c *staticIPs) UpdateStatus(staticIP *v1.StaticIP) (result *v1.StaticIP, err error) {
	result = &v1.StaticIP{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("staticips").
		Name(staticIP.Name).
		SubResource("status").
		Body(staticIP).
		Do().
		Into(result)
	return
}

// Delete takes name of the staticIP and deletes it. Returns an error if one occurs.
func (c *staticIPs) Delete(name string, options *metav1.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("staticips").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *staticIPs) DeleteCollection(options *metav1.DeleteOptions, listOptions metav1.ListOptions) error {
	var timeout time.Duration
	if listOptions.TimeoutSeconds != nil {
		timeout = time.Duration(*listOptions.TimeoutSeconds) * time.Second
	}
	return c.client.Delete().
		Namespace(c.ns).
		Resource("staticips").
		VersionedParams(&listOptions, scheme.ParameterCodec).
		Timeout(timeout).
		Body(options).
		Do().
		Error()
}

// Patch applies the patch and returns the patched staticIP.
func (c *staticIPs) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.StaticIP, err error) {
	result = &v1.StaticIP{}
	err = c.client.Patch(pt).
		Namespace(c.ns).
		Resource("staticips").
		SubResource(subresources...).
		Name(name).
		Body(data).
		Do().
		Into(result)
	return
}
