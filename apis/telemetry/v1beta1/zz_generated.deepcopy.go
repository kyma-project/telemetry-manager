//go:build !ignore_autogenerated

/*
Copyright 2021.

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

// Code generated by controller-gen. DO NOT EDIT.

package v1beta1

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AuthenticationOptions) DeepCopyInto(out *AuthenticationOptions) {
	*out = *in
	if in.Basic != nil {
		in, out := &in.Basic, &out.Basic
		*out = new(BasicAuthOptions)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AuthenticationOptions.
func (in *AuthenticationOptions) DeepCopy() *AuthenticationOptions {
	if in == nil {
		return nil
	}
	out := new(AuthenticationOptions)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *BasicAuthOptions) DeepCopyInto(out *BasicAuthOptions) {
	*out = *in
	in.User.DeepCopyInto(&out.User)
	in.Password.DeepCopyInto(&out.Password)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new BasicAuthOptions.
func (in *BasicAuthOptions) DeepCopy() *BasicAuthOptions {
	if in == nil {
		return nil
	}
	out := new(BasicAuthOptions)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DiagnosticMetrics) DeepCopyInto(out *DiagnosticMetrics) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DiagnosticMetrics.
func (in *DiagnosticMetrics) DeepCopy() *DiagnosticMetrics {
	if in == nil {
		return nil
	}
	out := new(DiagnosticMetrics)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Header) DeepCopyInto(out *Header) {
	*out = *in
	in.ValueType.DeepCopyInto(&out.ValueType)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Header.
func (in *Header) DeepCopy() *Header {
	if in == nil {
		return nil
	}
	out := new(Header)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *LogPipeline) DeepCopyInto(out *LogPipeline) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new LogPipeline.
func (in *LogPipeline) DeepCopy() *LogPipeline {
	if in == nil {
		return nil
	}
	out := new(LogPipeline)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *LogPipeline) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *LogPipelineFileMount) DeepCopyInto(out *LogPipelineFileMount) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new LogPipelineFileMount.
func (in *LogPipelineFileMount) DeepCopy() *LogPipelineFileMount {
	if in == nil {
		return nil
	}
	out := new(LogPipelineFileMount)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *LogPipelineFilter) DeepCopyInto(out *LogPipelineFilter) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new LogPipelineFilter.
func (in *LogPipelineFilter) DeepCopy() *LogPipelineFilter {
	if in == nil {
		return nil
	}
	out := new(LogPipelineFilter)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *LogPipelineHTTPOutput) DeepCopyInto(out *LogPipelineHTTPOutput) {
	*out = *in
	in.Host.DeepCopyInto(&out.Host)
	in.User.DeepCopyInto(&out.User)
	in.Password.DeepCopyInto(&out.Password)
	in.TLSConfig.DeepCopyInto(&out.TLSConfig)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new LogPipelineHTTPOutput.
func (in *LogPipelineHTTPOutput) DeepCopy() *LogPipelineHTTPOutput {
	if in == nil {
		return nil
	}
	out := new(LogPipelineHTTPOutput)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *LogPipelineInput) DeepCopyInto(out *LogPipelineInput) {
	*out = *in
	in.Runtime.DeepCopyInto(&out.Runtime)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new LogPipelineInput.
func (in *LogPipelineInput) DeepCopy() *LogPipelineInput {
	if in == nil {
		return nil
	}
	out := new(LogPipelineInput)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *LogPipelineInputContainers) DeepCopyInto(out *LogPipelineInputContainers) {
	*out = *in
	if in.Include != nil {
		in, out := &in.Include, &out.Include
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.Exclude != nil {
		in, out := &in.Exclude, &out.Exclude
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new LogPipelineInputContainers.
func (in *LogPipelineInputContainers) DeepCopy() *LogPipelineInputContainers {
	if in == nil {
		return nil
	}
	out := new(LogPipelineInputContainers)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *LogPipelineInputNamespaces) DeepCopyInto(out *LogPipelineInputNamespaces) {
	*out = *in
	if in.Include != nil {
		in, out := &in.Include, &out.Include
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.Exclude != nil {
		in, out := &in.Exclude, &out.Exclude
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new LogPipelineInputNamespaces.
func (in *LogPipelineInputNamespaces) DeepCopy() *LogPipelineInputNamespaces {
	if in == nil {
		return nil
	}
	out := new(LogPipelineInputNamespaces)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *LogPipelineList) DeepCopyInto(out *LogPipelineList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]LogPipeline, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new LogPipelineList.
func (in *LogPipelineList) DeepCopy() *LogPipelineList {
	if in == nil {
		return nil
	}
	out := new(LogPipelineList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *LogPipelineList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *LogPipelineOutput) DeepCopyInto(out *LogPipelineOutput) {
	*out = *in
	if in.HTTP != nil {
		in, out := &in.HTTP, &out.HTTP
		*out = new(LogPipelineHTTPOutput)
		(*in).DeepCopyInto(*out)
	}
	if in.OTLP != nil {
		in, out := &in.OTLP, &out.OTLP
		*out = new(OTLPOutput)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new LogPipelineOutput.
func (in *LogPipelineOutput) DeepCopy() *LogPipelineOutput {
	if in == nil {
		return nil
	}
	out := new(LogPipelineOutput)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *LogPipelineRuntimeInput) DeepCopyInto(out *LogPipelineRuntimeInput) {
	*out = *in
	if in.Enabled != nil {
		in, out := &in.Enabled, &out.Enabled
		*out = new(bool)
		**out = **in
	}
	in.Namespaces.DeepCopyInto(&out.Namespaces)
	in.Containers.DeepCopyInto(&out.Containers)
	if in.KeepOriginalBody != nil {
		in, out := &in.KeepOriginalBody, &out.KeepOriginalBody
		*out = new(bool)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new LogPipelineRuntimeInput.
func (in *LogPipelineRuntimeInput) DeepCopy() *LogPipelineRuntimeInput {
	if in == nil {
		return nil
	}
	out := new(LogPipelineRuntimeInput)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *LogPipelineSpec) DeepCopyInto(out *LogPipelineSpec) {
	*out = *in
	in.Input.DeepCopyInto(&out.Input)
	if in.Filters != nil {
		in, out := &in.Filters, &out.Filters
		*out = make([]LogPipelineFilter, len(*in))
		copy(*out, *in)
	}
	in.Output.DeepCopyInto(&out.Output)
	if in.Files != nil {
		in, out := &in.Files, &out.Files
		*out = make([]LogPipelineFileMount, len(*in))
		copy(*out, *in)
	}
	if in.Variables != nil {
		in, out := &in.Variables, &out.Variables
		*out = make([]LogPipelineVariableRef, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new LogPipelineSpec.
func (in *LogPipelineSpec) DeepCopy() *LogPipelineSpec {
	if in == nil {
		return nil
	}
	out := new(LogPipelineSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *LogPipelineStatus) DeepCopyInto(out *LogPipelineStatus) {
	*out = *in
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]v1.Condition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.UnsupportedMode != nil {
		in, out := &in.UnsupportedMode, &out.UnsupportedMode
		*out = new(bool)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new LogPipelineStatus.
func (in *LogPipelineStatus) DeepCopy() *LogPipelineStatus {
	if in == nil {
		return nil
	}
	out := new(LogPipelineStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *LogPipelineVariableRef) DeepCopyInto(out *LogPipelineVariableRef) {
	*out = *in
	in.ValueFrom.DeepCopyInto(&out.ValueFrom)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new LogPipelineVariableRef.
func (in *LogPipelineVariableRef) DeepCopy() *LogPipelineVariableRef {
	if in == nil {
		return nil
	}
	out := new(LogPipelineVariableRef)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MetricPipeline) DeepCopyInto(out *MetricPipeline) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MetricPipeline.
func (in *MetricPipeline) DeepCopy() *MetricPipeline {
	if in == nil {
		return nil
	}
	out := new(MetricPipeline)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *MetricPipeline) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MetricPipelineInput) DeepCopyInto(out *MetricPipelineInput) {
	*out = *in
	if in.Prometheus != nil {
		in, out := &in.Prometheus, &out.Prometheus
		*out = new(MetricPipelinePrometheusInput)
		(*in).DeepCopyInto(*out)
	}
	if in.Runtime != nil {
		in, out := &in.Runtime, &out.Runtime
		*out = new(MetricPipelineRuntimeInput)
		(*in).DeepCopyInto(*out)
	}
	if in.Istio != nil {
		in, out := &in.Istio, &out.Istio
		*out = new(MetricPipelineIstioInput)
		(*in).DeepCopyInto(*out)
	}
	if in.OTLP != nil {
		in, out := &in.OTLP, &out.OTLP
		*out = new(MetricPipelineOTLPInput)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MetricPipelineInput.
func (in *MetricPipelineInput) DeepCopy() *MetricPipelineInput {
	if in == nil {
		return nil
	}
	out := new(MetricPipelineInput)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MetricPipelineInputNamespaceSelector) DeepCopyInto(out *MetricPipelineInputNamespaceSelector) {
	*out = *in
	if in.Include != nil {
		in, out := &in.Include, &out.Include
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.Exclude != nil {
		in, out := &in.Exclude, &out.Exclude
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MetricPipelineInputNamespaceSelector.
func (in *MetricPipelineInputNamespaceSelector) DeepCopy() *MetricPipelineInputNamespaceSelector {
	if in == nil {
		return nil
	}
	out := new(MetricPipelineInputNamespaceSelector)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MetricPipelineIstioInput) DeepCopyInto(out *MetricPipelineIstioInput) {
	*out = *in
	if in.Namespaces != nil {
		in, out := &in.Namespaces, &out.Namespaces
		*out = new(MetricPipelineInputNamespaceSelector)
		(*in).DeepCopyInto(*out)
	}
	if in.DiagnosticMetrics != nil {
		in, out := &in.DiagnosticMetrics, &out.DiagnosticMetrics
		*out = new(DiagnosticMetrics)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MetricPipelineIstioInput.
func (in *MetricPipelineIstioInput) DeepCopy() *MetricPipelineIstioInput {
	if in == nil {
		return nil
	}
	out := new(MetricPipelineIstioInput)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MetricPipelineList) DeepCopyInto(out *MetricPipelineList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]MetricPipeline, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MetricPipelineList.
func (in *MetricPipelineList) DeepCopy() *MetricPipelineList {
	if in == nil {
		return nil
	}
	out := new(MetricPipelineList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *MetricPipelineList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MetricPipelineOTLPInput) DeepCopyInto(out *MetricPipelineOTLPInput) {
	*out = *in
	if in.Namespaces != nil {
		in, out := &in.Namespaces, &out.Namespaces
		*out = new(MetricPipelineInputNamespaceSelector)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MetricPipelineOTLPInput.
func (in *MetricPipelineOTLPInput) DeepCopy() *MetricPipelineOTLPInput {
	if in == nil {
		return nil
	}
	out := new(MetricPipelineOTLPInput)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MetricPipelineOutput) DeepCopyInto(out *MetricPipelineOutput) {
	*out = *in
	if in.OTLP != nil {
		in, out := &in.OTLP, &out.OTLP
		*out = new(OTLPOutput)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MetricPipelineOutput.
func (in *MetricPipelineOutput) DeepCopy() *MetricPipelineOutput {
	if in == nil {
		return nil
	}
	out := new(MetricPipelineOutput)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MetricPipelinePrometheusInput) DeepCopyInto(out *MetricPipelinePrometheusInput) {
	*out = *in
	if in.Namespaces != nil {
		in, out := &in.Namespaces, &out.Namespaces
		*out = new(MetricPipelineInputNamespaceSelector)
		(*in).DeepCopyInto(*out)
	}
	if in.DiagnosticMetrics != nil {
		in, out := &in.DiagnosticMetrics, &out.DiagnosticMetrics
		*out = new(DiagnosticMetrics)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MetricPipelinePrometheusInput.
func (in *MetricPipelinePrometheusInput) DeepCopy() *MetricPipelinePrometheusInput {
	if in == nil {
		return nil
	}
	out := new(MetricPipelinePrometheusInput)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MetricPipelineRuntimeInput) DeepCopyInto(out *MetricPipelineRuntimeInput) {
	*out = *in
	if in.Namespaces != nil {
		in, out := &in.Namespaces, &out.Namespaces
		*out = new(MetricPipelineInputNamespaceSelector)
		(*in).DeepCopyInto(*out)
	}
	if in.Resources != nil {
		in, out := &in.Resources, &out.Resources
		*out = new(MetricPipelineRuntimeInputResources)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MetricPipelineRuntimeInput.
func (in *MetricPipelineRuntimeInput) DeepCopy() *MetricPipelineRuntimeInput {
	if in == nil {
		return nil
	}
	out := new(MetricPipelineRuntimeInput)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MetricPipelineRuntimeInputResourceDisabledByDefault) DeepCopyInto(out *MetricPipelineRuntimeInputResourceDisabledByDefault) {
	*out = *in
	if in.Enabled != nil {
		in, out := &in.Enabled, &out.Enabled
		*out = new(bool)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MetricPipelineRuntimeInputResourceDisabledByDefault.
func (in *MetricPipelineRuntimeInputResourceDisabledByDefault) DeepCopy() *MetricPipelineRuntimeInputResourceDisabledByDefault {
	if in == nil {
		return nil
	}
	out := new(MetricPipelineRuntimeInputResourceDisabledByDefault)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MetricPipelineRuntimeInputResourceEnabledByDefault) DeepCopyInto(out *MetricPipelineRuntimeInputResourceEnabledByDefault) {
	*out = *in
	if in.Enabled != nil {
		in, out := &in.Enabled, &out.Enabled
		*out = new(bool)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MetricPipelineRuntimeInputResourceEnabledByDefault.
func (in *MetricPipelineRuntimeInputResourceEnabledByDefault) DeepCopy() *MetricPipelineRuntimeInputResourceEnabledByDefault {
	if in == nil {
		return nil
	}
	out := new(MetricPipelineRuntimeInputResourceEnabledByDefault)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MetricPipelineRuntimeInputResources) DeepCopyInto(out *MetricPipelineRuntimeInputResources) {
	*out = *in
	if in.Pod != nil {
		in, out := &in.Pod, &out.Pod
		*out = new(MetricPipelineRuntimeInputResourceEnabledByDefault)
		(*in).DeepCopyInto(*out)
	}
	if in.Container != nil {
		in, out := &in.Container, &out.Container
		*out = new(MetricPipelineRuntimeInputResourceEnabledByDefault)
		(*in).DeepCopyInto(*out)
	}
	if in.Node != nil {
		in, out := &in.Node, &out.Node
		*out = new(MetricPipelineRuntimeInputResourceDisabledByDefault)
		(*in).DeepCopyInto(*out)
	}
	if in.Volume != nil {
		in, out := &in.Volume, &out.Volume
		*out = new(MetricPipelineRuntimeInputResourceDisabledByDefault)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MetricPipelineRuntimeInputResources.
func (in *MetricPipelineRuntimeInputResources) DeepCopy() *MetricPipelineRuntimeInputResources {
	if in == nil {
		return nil
	}
	out := new(MetricPipelineRuntimeInputResources)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MetricPipelineSpec) DeepCopyInto(out *MetricPipelineSpec) {
	*out = *in
	in.Input.DeepCopyInto(&out.Input)
	in.Output.DeepCopyInto(&out.Output)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MetricPipelineSpec.
func (in *MetricPipelineSpec) DeepCopy() *MetricPipelineSpec {
	if in == nil {
		return nil
	}
	out := new(MetricPipelineSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MetricPipelineStatus) DeepCopyInto(out *MetricPipelineStatus) {
	*out = *in
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]v1.Condition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MetricPipelineStatus.
func (in *MetricPipelineStatus) DeepCopy() *MetricPipelineStatus {
	if in == nil {
		return nil
	}
	out := new(MetricPipelineStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *OTLPOutput) DeepCopyInto(out *OTLPOutput) {
	*out = *in
	in.Endpoint.DeepCopyInto(&out.Endpoint)
	if in.Authentication != nil {
		in, out := &in.Authentication, &out.Authentication
		*out = new(AuthenticationOptions)
		(*in).DeepCopyInto(*out)
	}
	if in.Headers != nil {
		in, out := &in.Headers, &out.Headers
		*out = make([]Header, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.TLS != nil {
		in, out := &in.TLS, &out.TLS
		*out = new(OutputTLS)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new OTLPOutput.
func (in *OTLPOutput) DeepCopy() *OTLPOutput {
	if in == nil {
		return nil
	}
	out := new(OTLPOutput)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *OutputTLS) DeepCopyInto(out *OutputTLS) {
	*out = *in
	if in.CA != nil {
		in, out := &in.CA, &out.CA
		*out = new(ValueType)
		(*in).DeepCopyInto(*out)
	}
	if in.Cert != nil {
		in, out := &in.Cert, &out.Cert
		*out = new(ValueType)
		(*in).DeepCopyInto(*out)
	}
	if in.Key != nil {
		in, out := &in.Key, &out.Key
		*out = new(ValueType)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new OutputTLS.
func (in *OutputTLS) DeepCopy() *OutputTLS {
	if in == nil {
		return nil
	}
	out := new(OutputTLS)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SecretKeyRef) DeepCopyInto(out *SecretKeyRef) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SecretKeyRef.
func (in *SecretKeyRef) DeepCopy() *SecretKeyRef {
	if in == nil {
		return nil
	}
	out := new(SecretKeyRef)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *TracePipeline) DeepCopyInto(out *TracePipeline) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new TracePipeline.
func (in *TracePipeline) DeepCopy() *TracePipeline {
	if in == nil {
		return nil
	}
	out := new(TracePipeline)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *TracePipeline) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *TracePipelineList) DeepCopyInto(out *TracePipelineList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]TracePipeline, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new TracePipelineList.
func (in *TracePipelineList) DeepCopy() *TracePipelineList {
	if in == nil {
		return nil
	}
	out := new(TracePipelineList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *TracePipelineList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *TracePipelineOutput) DeepCopyInto(out *TracePipelineOutput) {
	*out = *in
	if in.OTLP != nil {
		in, out := &in.OTLP, &out.OTLP
		*out = new(OTLPOutput)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new TracePipelineOutput.
func (in *TracePipelineOutput) DeepCopy() *TracePipelineOutput {
	if in == nil {
		return nil
	}
	out := new(TracePipelineOutput)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *TracePipelineSpec) DeepCopyInto(out *TracePipelineSpec) {
	*out = *in
	in.Output.DeepCopyInto(&out.Output)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new TracePipelineSpec.
func (in *TracePipelineSpec) DeepCopy() *TracePipelineSpec {
	if in == nil {
		return nil
	}
	out := new(TracePipelineSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *TracePipelineStatus) DeepCopyInto(out *TracePipelineStatus) {
	*out = *in
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]v1.Condition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new TracePipelineStatus.
func (in *TracePipelineStatus) DeepCopy() *TracePipelineStatus {
	if in == nil {
		return nil
	}
	out := new(TracePipelineStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ValueFromSource) DeepCopyInto(out *ValueFromSource) {
	*out = *in
	if in.SecretKeyRef != nil {
		in, out := &in.SecretKeyRef, &out.SecretKeyRef
		*out = new(SecretKeyRef)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ValueFromSource.
func (in *ValueFromSource) DeepCopy() *ValueFromSource {
	if in == nil {
		return nil
	}
	out := new(ValueFromSource)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ValueType) DeepCopyInto(out *ValueType) {
	*out = *in
	if in.ValueFrom != nil {
		in, out := &in.ValueFrom, &out.ValueFrom
		*out = new(ValueFromSource)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ValueType.
func (in *ValueType) DeepCopy() *ValueType {
	if in == nil {
		return nil
	}
	out := new(ValueType)
	in.DeepCopyInto(out)
	return out
}
