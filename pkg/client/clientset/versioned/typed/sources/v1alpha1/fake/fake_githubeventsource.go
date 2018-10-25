/*
Copyright 2018 The Knative Authors

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

package fake

import (
	v1alpha1 "github.com/knative/eventing-sources/pkg/apis/sources/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeGitHubEventSources implements GitHubEventSourceInterface
type FakeGitHubEventSources struct {
	Fake *FakeSourcesV1alpha1
	ns   string
}

var githubeventsourcesResource = schema.GroupVersionResource{Group: "sources.eventing.knative.dev", Version: "v1alpha1", Resource: "githubeventsources"}

var githubeventsourcesKind = schema.GroupVersionKind{Group: "sources.eventing.knative.dev", Version: "v1alpha1", Kind: "GitHubEventSource"}

// Get takes name of the gitHubEventSource, and returns the corresponding gitHubEventSource object, and an error if there is any.
func (c *FakeGitHubEventSources) Get(name string, options v1.GetOptions) (result *v1alpha1.GitHubEventSource, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(githubeventsourcesResource, c.ns, name), &v1alpha1.GitHubEventSource{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.GitHubEventSource), err
}

// List takes label and field selectors, and returns the list of GitHubEventSources that match those selectors.
func (c *FakeGitHubEventSources) List(opts v1.ListOptions) (result *v1alpha1.GitHubEventSourceList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(githubeventsourcesResource, githubeventsourcesKind, c.ns, opts), &v1alpha1.GitHubEventSourceList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1alpha1.GitHubEventSourceList{ListMeta: obj.(*v1alpha1.GitHubEventSourceList).ListMeta}
	for _, item := range obj.(*v1alpha1.GitHubEventSourceList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested gitHubEventSources.
func (c *FakeGitHubEventSources) Watch(opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(githubeventsourcesResource, c.ns, opts))

}

// Create takes the representation of a gitHubEventSource and creates it.  Returns the server's representation of the gitHubEventSource, and an error, if there is any.
func (c *FakeGitHubEventSources) Create(gitHubEventSource *v1alpha1.GitHubEventSource) (result *v1alpha1.GitHubEventSource, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(githubeventsourcesResource, c.ns, gitHubEventSource), &v1alpha1.GitHubEventSource{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.GitHubEventSource), err
}

// Update takes the representation of a gitHubEventSource and updates it. Returns the server's representation of the gitHubEventSource, and an error, if there is any.
func (c *FakeGitHubEventSources) Update(gitHubEventSource *v1alpha1.GitHubEventSource) (result *v1alpha1.GitHubEventSource, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(githubeventsourcesResource, c.ns, gitHubEventSource), &v1alpha1.GitHubEventSource{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.GitHubEventSource), err
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *FakeGitHubEventSources) UpdateStatus(gitHubEventSource *v1alpha1.GitHubEventSource) (*v1alpha1.GitHubEventSource, error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateSubresourceAction(githubeventsourcesResource, "status", c.ns, gitHubEventSource), &v1alpha1.GitHubEventSource{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.GitHubEventSource), err
}

// Delete takes name of the gitHubEventSource and deletes it. Returns an error if one occurs.
func (c *FakeGitHubEventSources) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(githubeventsourcesResource, c.ns, name), &v1alpha1.GitHubEventSource{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeGitHubEventSources) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(githubeventsourcesResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &v1alpha1.GitHubEventSourceList{})
	return err
}

// Patch applies the patch and returns the patched gitHubEventSource.
func (c *FakeGitHubEventSources) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.GitHubEventSource, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(githubeventsourcesResource, c.ns, name, data, subresources...), &v1alpha1.GitHubEventSource{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.GitHubEventSource), err
}
