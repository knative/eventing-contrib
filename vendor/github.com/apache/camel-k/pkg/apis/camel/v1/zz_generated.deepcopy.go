// +build !ignore_autogenerated

// Code generated by operator-sdk. DO NOT EDIT.

package v1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Artifact) DeepCopyInto(out *Artifact) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Artifact.
func (in *Artifact) DeepCopy() *Artifact {
	if in == nil {
		return nil
	}
	out := new(Artifact)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *BaseTask) DeepCopyInto(out *BaseTask) {
	*out = *in
	if in.Affinity != nil {
		in, out := &in.Affinity, &out.Affinity
		*out = new(corev1.Affinity)
		(*in).DeepCopyInto(*out)
	}
	if in.Volumes != nil {
		in, out := &in.Volumes, &out.Volumes
		*out = make([]corev1.Volume, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.VolumeMounts != nil {
		in, out := &in.VolumeMounts, &out.VolumeMounts
		*out = make([]corev1.VolumeMount, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new BaseTask.
func (in *BaseTask) DeepCopy() *BaseTask {
	if in == nil {
		return nil
	}
	out := new(BaseTask)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Build) DeepCopyInto(out *Build) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Build.
func (in *Build) DeepCopy() *Build {
	if in == nil {
		return nil
	}
	out := new(Build)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *Build) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *BuildCondition) DeepCopyInto(out *BuildCondition) {
	*out = *in
	in.LastUpdateTime.DeepCopyInto(&out.LastUpdateTime)
	in.LastTransitionTime.DeepCopyInto(&out.LastTransitionTime)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new BuildCondition.
func (in *BuildCondition) DeepCopy() *BuildCondition {
	if in == nil {
		return nil
	}
	out := new(BuildCondition)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *BuildList) DeepCopyInto(out *BuildList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]Build, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new BuildList.
func (in *BuildList) DeepCopy() *BuildList {
	if in == nil {
		return nil
	}
	out := new(BuildList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *BuildList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *BuildSpec) DeepCopyInto(out *BuildSpec) {
	*out = *in
	if in.Tasks != nil {
		in, out := &in.Tasks, &out.Tasks
		*out = make([]Task, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new BuildSpec.
func (in *BuildSpec) DeepCopy() *BuildSpec {
	if in == nil {
		return nil
	}
	out := new(BuildSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *BuildStatus) DeepCopyInto(out *BuildStatus) {
	*out = *in
	if in.Artifacts != nil {
		in, out := &in.Artifacts, &out.Artifacts
		*out = make([]Artifact, len(*in))
		copy(*out, *in)
	}
	if in.Failure != nil {
		in, out := &in.Failure, &out.Failure
		*out = new(Failure)
		(*in).DeepCopyInto(*out)
	}
	in.StartedAt.DeepCopyInto(&out.StartedAt)
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]BuildCondition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new BuildStatus.
func (in *BuildStatus) DeepCopy() *BuildStatus {
	if in == nil {
		return nil
	}
	out := new(BuildStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *BuilderTask) DeepCopyInto(out *BuilderTask) {
	*out = *in
	in.BaseTask.DeepCopyInto(&out.BaseTask)
	in.Meta.DeepCopyInto(&out.Meta)
	in.Runtime.DeepCopyInto(&out.Runtime)
	if in.Sources != nil {
		in, out := &in.Sources, &out.Sources
		*out = make([]SourceSpec, len(*in))
		copy(*out, *in)
	}
	if in.Resources != nil {
		in, out := &in.Resources, &out.Resources
		*out = make([]ResourceSpec, len(*in))
		copy(*out, *in)
	}
	if in.Dependencies != nil {
		in, out := &in.Dependencies, &out.Dependencies
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.Steps != nil {
		in, out := &in.Steps, &out.Steps
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	in.Maven.DeepCopyInto(&out.Maven)
	if in.Properties != nil {
		in, out := &in.Properties, &out.Properties
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	out.Timeout = in.Timeout
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new BuilderTask.
func (in *BuilderTask) DeepCopy() *BuilderTask {
	if in == nil {
		return nil
	}
	out := new(BuilderTask)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CamelArtifact) DeepCopyInto(out *CamelArtifact) {
	*out = *in
	in.CamelArtifactDependency.DeepCopyInto(&out.CamelArtifactDependency)
	if in.Schemes != nil {
		in, out := &in.Schemes, &out.Schemes
		*out = make([]CamelScheme, len(*in))
		copy(*out, *in)
	}
	if in.Languages != nil {
		in, out := &in.Languages, &out.Languages
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.DataFormats != nil {
		in, out := &in.DataFormats, &out.DataFormats
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.Dependencies != nil {
		in, out := &in.Dependencies, &out.Dependencies
		*out = make([]CamelArtifact, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.JavaTypes != nil {
		in, out := &in.JavaTypes, &out.JavaTypes
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CamelArtifact.
func (in *CamelArtifact) DeepCopy() *CamelArtifact {
	if in == nil {
		return nil
	}
	out := new(CamelArtifact)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CamelArtifactDependency) DeepCopyInto(out *CamelArtifactDependency) {
	*out = *in
	out.MavenArtifact = in.MavenArtifact
	if in.Exclusions != nil {
		in, out := &in.Exclusions, &out.Exclusions
		*out = make([]CamelArtifactExclusion, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CamelArtifactDependency.
func (in *CamelArtifactDependency) DeepCopy() *CamelArtifactDependency {
	if in == nil {
		return nil
	}
	out := new(CamelArtifactDependency)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CamelArtifactExclusion) DeepCopyInto(out *CamelArtifactExclusion) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CamelArtifactExclusion.
func (in *CamelArtifactExclusion) DeepCopy() *CamelArtifactExclusion {
	if in == nil {
		return nil
	}
	out := new(CamelArtifactExclusion)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CamelCatalog) DeepCopyInto(out *CamelCatalog) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.Status = in.Status
	in.Spec.DeepCopyInto(&out.Spec)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CamelCatalog.
func (in *CamelCatalog) DeepCopy() *CamelCatalog {
	if in == nil {
		return nil
	}
	out := new(CamelCatalog)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *CamelCatalog) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CamelCatalogList) DeepCopyInto(out *CamelCatalogList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]CamelCatalog, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CamelCatalogList.
func (in *CamelCatalogList) DeepCopy() *CamelCatalogList {
	if in == nil {
		return nil
	}
	out := new(CamelCatalogList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *CamelCatalogList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CamelCatalogSpec) DeepCopyInto(out *CamelCatalogSpec) {
	*out = *in
	in.Runtime.DeepCopyInto(&out.Runtime)
	if in.Artifacts != nil {
		in, out := &in.Artifacts, &out.Artifacts
		*out = make(map[string]CamelArtifact, len(*in))
		for key, val := range *in {
			(*out)[key] = *val.DeepCopy()
		}
	}
	if in.Loaders != nil {
		in, out := &in.Loaders, &out.Loaders
		*out = make(map[string]CamelLoader, len(*in))
		for key, val := range *in {
			(*out)[key] = *val.DeepCopy()
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CamelCatalogSpec.
func (in *CamelCatalogSpec) DeepCopy() *CamelCatalogSpec {
	if in == nil {
		return nil
	}
	out := new(CamelCatalogSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CamelCatalogStatus) DeepCopyInto(out *CamelCatalogStatus) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CamelCatalogStatus.
func (in *CamelCatalogStatus) DeepCopy() *CamelCatalogStatus {
	if in == nil {
		return nil
	}
	out := new(CamelCatalogStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CamelLoader) DeepCopyInto(out *CamelLoader) {
	*out = *in
	out.MavenArtifact = in.MavenArtifact
	if in.Languages != nil {
		in, out := &in.Languages, &out.Languages
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.Dependencies != nil {
		in, out := &in.Dependencies, &out.Dependencies
		*out = make([]MavenArtifact, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CamelLoader.
func (in *CamelLoader) DeepCopy() *CamelLoader {
	if in == nil {
		return nil
	}
	out := new(CamelLoader)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CamelScheme) DeepCopyInto(out *CamelScheme) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CamelScheme.
func (in *CamelScheme) DeepCopy() *CamelScheme {
	if in == nil {
		return nil
	}
	out := new(CamelScheme)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ConfigurationSpec) DeepCopyInto(out *ConfigurationSpec) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ConfigurationSpec.
func (in *ConfigurationSpec) DeepCopy() *ConfigurationSpec {
	if in == nil {
		return nil
	}
	out := new(ConfigurationSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ContainerTask) DeepCopyInto(out *ContainerTask) {
	*out = *in
	in.BaseTask.DeepCopyInto(&out.BaseTask)
	if in.Command != nil {
		in, out := &in.Command, &out.Command
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.Args != nil {
		in, out := &in.Args, &out.Args
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.Env != nil {
		in, out := &in.Env, &out.Env
		*out = make([]corev1.EnvVar, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ContainerTask.
func (in *ContainerTask) DeepCopy() *ContainerTask {
	if in == nil {
		return nil
	}
	out := new(ContainerTask)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DataSpec) DeepCopyInto(out *DataSpec) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DataSpec.
func (in *DataSpec) DeepCopy() *DataSpec {
	if in == nil {
		return nil
	}
	out := new(DataSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Failure) DeepCopyInto(out *Failure) {
	*out = *in
	in.Time.DeepCopyInto(&out.Time)
	in.Recovery.DeepCopyInto(&out.Recovery)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Failure.
func (in *Failure) DeepCopy() *Failure {
	if in == nil {
		return nil
	}
	out := new(Failure)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *FailureRecovery) DeepCopyInto(out *FailureRecovery) {
	*out = *in
	in.AttemptTime.DeepCopyInto(&out.AttemptTime)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new FailureRecovery.
func (in *FailureRecovery) DeepCopy() *FailureRecovery {
	if in == nil {
		return nil
	}
	out := new(FailureRecovery)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ImageTask) DeepCopyInto(out *ImageTask) {
	*out = *in
	in.ContainerTask.DeepCopyInto(&out.ContainerTask)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ImageTask.
func (in *ImageTask) DeepCopy() *ImageTask {
	if in == nil {
		return nil
	}
	out := new(ImageTask)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Integration) DeepCopyInto(out *Integration) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Integration.
func (in *Integration) DeepCopy() *Integration {
	if in == nil {
		return nil
	}
	out := new(Integration)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *Integration) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *IntegrationCondition) DeepCopyInto(out *IntegrationCondition) {
	*out = *in
	in.LastUpdateTime.DeepCopyInto(&out.LastUpdateTime)
	in.LastTransitionTime.DeepCopyInto(&out.LastTransitionTime)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new IntegrationCondition.
func (in *IntegrationCondition) DeepCopy() *IntegrationCondition {
	if in == nil {
		return nil
	}
	out := new(IntegrationCondition)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *IntegrationKit) DeepCopyInto(out *IntegrationKit) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new IntegrationKit.
func (in *IntegrationKit) DeepCopy() *IntegrationKit {
	if in == nil {
		return nil
	}
	out := new(IntegrationKit)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *IntegrationKit) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *IntegrationKitCondition) DeepCopyInto(out *IntegrationKitCondition) {
	*out = *in
	in.LastUpdateTime.DeepCopyInto(&out.LastUpdateTime)
	in.LastTransitionTime.DeepCopyInto(&out.LastTransitionTime)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new IntegrationKitCondition.
func (in *IntegrationKitCondition) DeepCopy() *IntegrationKitCondition {
	if in == nil {
		return nil
	}
	out := new(IntegrationKitCondition)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *IntegrationKitList) DeepCopyInto(out *IntegrationKitList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]IntegrationKit, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new IntegrationKitList.
func (in *IntegrationKitList) DeepCopy() *IntegrationKitList {
	if in == nil {
		return nil
	}
	out := new(IntegrationKitList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *IntegrationKitList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *IntegrationKitSpec) DeepCopyInto(out *IntegrationKitSpec) {
	*out = *in
	if in.Dependencies != nil {
		in, out := &in.Dependencies, &out.Dependencies
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.Traits != nil {
		in, out := &in.Traits, &out.Traits
		*out = make(map[string]TraitSpec, len(*in))
		for key, val := range *in {
			(*out)[key] = *val.DeepCopy()
		}
	}
	if in.Configuration != nil {
		in, out := &in.Configuration, &out.Configuration
		*out = make([]ConfigurationSpec, len(*in))
		copy(*out, *in)
	}
	if in.Repositories != nil {
		in, out := &in.Repositories, &out.Repositories
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new IntegrationKitSpec.
func (in *IntegrationKitSpec) DeepCopy() *IntegrationKitSpec {
	if in == nil {
		return nil
	}
	out := new(IntegrationKitSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *IntegrationKitStatus) DeepCopyInto(out *IntegrationKitStatus) {
	*out = *in
	if in.Artifacts != nil {
		in, out := &in.Artifacts, &out.Artifacts
		*out = make([]Artifact, len(*in))
		copy(*out, *in)
	}
	if in.Failure != nil {
		in, out := &in.Failure, &out.Failure
		*out = new(Failure)
		(*in).DeepCopyInto(*out)
	}
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]IntegrationKitCondition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new IntegrationKitStatus.
func (in *IntegrationKitStatus) DeepCopy() *IntegrationKitStatus {
	if in == nil {
		return nil
	}
	out := new(IntegrationKitStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *IntegrationList) DeepCopyInto(out *IntegrationList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]Integration, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new IntegrationList.
func (in *IntegrationList) DeepCopy() *IntegrationList {
	if in == nil {
		return nil
	}
	out := new(IntegrationList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *IntegrationList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *IntegrationPlatform) DeepCopyInto(out *IntegrationPlatform) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new IntegrationPlatform.
func (in *IntegrationPlatform) DeepCopy() *IntegrationPlatform {
	if in == nil {
		return nil
	}
	out := new(IntegrationPlatform)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *IntegrationPlatform) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *IntegrationPlatformBuildSpec) DeepCopyInto(out *IntegrationPlatformBuildSpec) {
	*out = *in
	if in.Properties != nil {
		in, out := &in.Properties, &out.Properties
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	out.Registry = in.Registry
	if in.Timeout != nil {
		in, out := &in.Timeout, &out.Timeout
		*out = new(metav1.Duration)
		**out = **in
	}
	in.Maven.DeepCopyInto(&out.Maven)
	if in.KanikoBuildCache != nil {
		in, out := &in.KanikoBuildCache, &out.KanikoBuildCache
		*out = new(bool)
		**out = **in
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new IntegrationPlatformBuildSpec.
func (in *IntegrationPlatformBuildSpec) DeepCopy() *IntegrationPlatformBuildSpec {
	if in == nil {
		return nil
	}
	out := new(IntegrationPlatformBuildSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *IntegrationPlatformCondition) DeepCopyInto(out *IntegrationPlatformCondition) {
	*out = *in
	in.LastUpdateTime.DeepCopyInto(&out.LastUpdateTime)
	in.LastTransitionTime.DeepCopyInto(&out.LastTransitionTime)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new IntegrationPlatformCondition.
func (in *IntegrationPlatformCondition) DeepCopy() *IntegrationPlatformCondition {
	if in == nil {
		return nil
	}
	out := new(IntegrationPlatformCondition)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *IntegrationPlatformList) DeepCopyInto(out *IntegrationPlatformList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]IntegrationPlatform, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new IntegrationPlatformList.
func (in *IntegrationPlatformList) DeepCopy() *IntegrationPlatformList {
	if in == nil {
		return nil
	}
	out := new(IntegrationPlatformList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *IntegrationPlatformList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *IntegrationPlatformRegistrySpec) DeepCopyInto(out *IntegrationPlatformRegistrySpec) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new IntegrationPlatformRegistrySpec.
func (in *IntegrationPlatformRegistrySpec) DeepCopy() *IntegrationPlatformRegistrySpec {
	if in == nil {
		return nil
	}
	out := new(IntegrationPlatformRegistrySpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *IntegrationPlatformResourcesSpec) DeepCopyInto(out *IntegrationPlatformResourcesSpec) {
	*out = *in
	if in.Kits != nil {
		in, out := &in.Kits, &out.Kits
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new IntegrationPlatformResourcesSpec.
func (in *IntegrationPlatformResourcesSpec) DeepCopy() *IntegrationPlatformResourcesSpec {
	if in == nil {
		return nil
	}
	out := new(IntegrationPlatformResourcesSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *IntegrationPlatformSpec) DeepCopyInto(out *IntegrationPlatformSpec) {
	*out = *in
	in.Build.DeepCopyInto(&out.Build)
	in.Resources.DeepCopyInto(&out.Resources)
	if in.Traits != nil {
		in, out := &in.Traits, &out.Traits
		*out = make(map[string]TraitSpec, len(*in))
		for key, val := range *in {
			(*out)[key] = *val.DeepCopy()
		}
	}
	if in.Configuration != nil {
		in, out := &in.Configuration, &out.Configuration
		*out = make([]ConfigurationSpec, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new IntegrationPlatformSpec.
func (in *IntegrationPlatformSpec) DeepCopy() *IntegrationPlatformSpec {
	if in == nil {
		return nil
	}
	out := new(IntegrationPlatformSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *IntegrationPlatformStatus) DeepCopyInto(out *IntegrationPlatformStatus) {
	*out = *in
	in.IntegrationPlatformSpec.DeepCopyInto(&out.IntegrationPlatformSpec)
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]IntegrationPlatformCondition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new IntegrationPlatformStatus.
func (in *IntegrationPlatformStatus) DeepCopy() *IntegrationPlatformStatus {
	if in == nil {
		return nil
	}
	out := new(IntegrationPlatformStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *IntegrationSpec) DeepCopyInto(out *IntegrationSpec) {
	*out = *in
	if in.Replicas != nil {
		in, out := &in.Replicas, &out.Replicas
		*out = new(int32)
		**out = **in
	}
	if in.Sources != nil {
		in, out := &in.Sources, &out.Sources
		*out = make([]SourceSpec, len(*in))
		copy(*out, *in)
	}
	if in.Resources != nil {
		in, out := &in.Resources, &out.Resources
		*out = make([]ResourceSpec, len(*in))
		copy(*out, *in)
	}
	if in.Dependencies != nil {
		in, out := &in.Dependencies, &out.Dependencies
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.Traits != nil {
		in, out := &in.Traits, &out.Traits
		*out = make(map[string]TraitSpec, len(*in))
		for key, val := range *in {
			(*out)[key] = *val.DeepCopy()
		}
	}
	if in.Configuration != nil {
		in, out := &in.Configuration, &out.Configuration
		*out = make([]ConfigurationSpec, len(*in))
		copy(*out, *in)
	}
	if in.Repositories != nil {
		in, out := &in.Repositories, &out.Repositories
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new IntegrationSpec.
func (in *IntegrationSpec) DeepCopy() *IntegrationSpec {
	if in == nil {
		return nil
	}
	out := new(IntegrationSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *IntegrationStatus) DeepCopyInto(out *IntegrationStatus) {
	*out = *in
	if in.Dependencies != nil {
		in, out := &in.Dependencies, &out.Dependencies
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.GeneratedSources != nil {
		in, out := &in.GeneratedSources, &out.GeneratedSources
		*out = make([]SourceSpec, len(*in))
		copy(*out, *in)
	}
	if in.GeneratedResources != nil {
		in, out := &in.GeneratedResources, &out.GeneratedResources
		*out = make([]ResourceSpec, len(*in))
		copy(*out, *in)
	}
	if in.Failure != nil {
		in, out := &in.Failure, &out.Failure
		*out = new(Failure)
		(*in).DeepCopyInto(*out)
	}
	if in.Configuration != nil {
		in, out := &in.Configuration, &out.Configuration
		*out = make([]ConfigurationSpec, len(*in))
		copy(*out, *in)
	}
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]IntegrationCondition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.Replicas != nil {
		in, out := &in.Replicas, &out.Replicas
		*out = new(int32)
		**out = **in
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new IntegrationStatus.
func (in *IntegrationStatus) DeepCopy() *IntegrationStatus {
	if in == nil {
		return nil
	}
	out := new(IntegrationStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MavenArtifact) DeepCopyInto(out *MavenArtifact) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MavenArtifact.
func (in *MavenArtifact) DeepCopy() *MavenArtifact {
	if in == nil {
		return nil
	}
	out := new(MavenArtifact)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MavenSpec) DeepCopyInto(out *MavenSpec) {
	*out = *in
	in.Settings.DeepCopyInto(&out.Settings)
	if in.Timeout != nil {
		in, out := &in.Timeout, &out.Timeout
		*out = new(metav1.Duration)
		**out = **in
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MavenSpec.
func (in *MavenSpec) DeepCopy() *MavenSpec {
	if in == nil {
		return nil
	}
	out := new(MavenSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ResourceSpec) DeepCopyInto(out *ResourceSpec) {
	*out = *in
	out.DataSpec = in.DataSpec
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ResourceSpec.
func (in *ResourceSpec) DeepCopy() *ResourceSpec {
	if in == nil {
		return nil
	}
	out := new(ResourceSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *RuntimeSpec) DeepCopyInto(out *RuntimeSpec) {
	*out = *in
	if in.Dependencies != nil {
		in, out := &in.Dependencies, &out.Dependencies
		*out = make([]MavenArtifact, len(*in))
		copy(*out, *in)
	}
	if in.Metadata != nil {
		in, out := &in.Metadata, &out.Metadata
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new RuntimeSpec.
func (in *RuntimeSpec) DeepCopy() *RuntimeSpec {
	if in == nil {
		return nil
	}
	out := new(RuntimeSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SourceSpec) DeepCopyInto(out *SourceSpec) {
	*out = *in
	out.DataSpec = in.DataSpec
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SourceSpec.
func (in *SourceSpec) DeepCopy() *SourceSpec {
	if in == nil {
		return nil
	}
	out := new(SourceSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Task) DeepCopyInto(out *Task) {
	*out = *in
	if in.Builder != nil {
		in, out := &in.Builder, &out.Builder
		*out = new(BuilderTask)
		(*in).DeepCopyInto(*out)
	}
	if in.Image != nil {
		in, out := &in.Image, &out.Image
		*out = new(ImageTask)
		(*in).DeepCopyInto(*out)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Task.
func (in *Task) DeepCopy() *Task {
	if in == nil {
		return nil
	}
	out := new(Task)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *TraitSpec) DeepCopyInto(out *TraitSpec) {
	*out = *in
	if in.Configuration != nil {
		in, out := &in.Configuration, &out.Configuration
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new TraitSpec.
func (in *TraitSpec) DeepCopy() *TraitSpec {
	if in == nil {
		return nil
	}
	out := new(TraitSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ValueSource) DeepCopyInto(out *ValueSource) {
	*out = *in
	if in.ConfigMapKeyRef != nil {
		in, out := &in.ConfigMapKeyRef, &out.ConfigMapKeyRef
		*out = new(corev1.ConfigMapKeySelector)
		(*in).DeepCopyInto(*out)
	}
	if in.SecretKeyRef != nil {
		in, out := &in.SecretKeyRef, &out.SecretKeyRef
		*out = new(corev1.SecretKeySelector)
		(*in).DeepCopyInto(*out)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ValueSource.
func (in *ValueSource) DeepCopy() *ValueSource {
	if in == nil {
		return nil
	}
	out := new(ValueSource)
	in.DeepCopyInto(out)
	return out
}
