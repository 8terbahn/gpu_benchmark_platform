package v1alpha1

import (
	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
)

// GPUBenchmarkJob Methods

func (mg *GPUBenchmarkJob) SetConditions(c ...xpv1.Condition) {
	mg.Status.SetConditions(c...)
}

func (mg *GPUBenchmarkJob) GetCondition(ct xpv1.ConditionType) xpv1.Condition {
	return mg.Status.GetCondition(ct)
}

func (mg *GPUBenchmarkJob) GetProviderConfigReference() *xpv1.Reference {
	return mg.Spec.ProviderConfigReference
}

func (mg *GPUBenchmarkJob) SetProviderConfigReference(r *xpv1.Reference) {
	mg.Spec.ProviderConfigReference = r
}

func (mg *GPUBenchmarkJob) GetProviderReference() *xpv1.Reference {
	return mg.Spec.ProviderReference
}

func (mg *GPUBenchmarkJob) SetProviderReference(r *xpv1.Reference) {
	mg.Spec.ProviderReference = r
}

func (mg *GPUBenchmarkJob) GetDeletionPolicy() xpv1.DeletionPolicy {
	return mg.Spec.DeletionPolicy
}

func (mg *GPUBenchmarkJob) SetDeletionPolicy(r xpv1.DeletionPolicy) {
	mg.Spec.DeletionPolicy = r
}

func (mg *GPUBenchmarkJob) GetManagementPolicies() xpv1.ManagementPolicies {
	return mg.Spec.ManagementPolicies
}

func (mg *GPUBenchmarkJob) SetManagementPolicies(r xpv1.ManagementPolicies) {
	mg.Spec.ManagementPolicies = r
}

func (mg *GPUBenchmarkJob) GetPublishConnectionDetailsTo() *xpv1.PublishConnectionDetailsTo {
	return mg.Spec.PublishConnectionDetailsTo
}

func (mg *GPUBenchmarkJob) SetPublishConnectionDetailsTo(r *xpv1.PublishConnectionDetailsTo) {
	mg.Spec.PublishConnectionDetailsTo = r
}

func (mg *GPUBenchmarkJob) GetWriteConnectionSecretToReference() *xpv1.SecretReference {
	return mg.Spec.WriteConnectionSecretToReference
}

func (mg *GPUBenchmarkJob) SetWriteConnectionSecretToReference(r *xpv1.SecretReference) {
	mg.Spec.WriteConnectionSecretToReference = r
}

// ProviderConfig Methods

func (mg *ProviderConfig) GetCondition(ct xpv1.ConditionType) xpv1.Condition {
	return mg.Status.GetCondition(ct)
}

func (mg *ProviderConfig) SetConditions(c ...xpv1.Condition) {
	mg.Status.SetConditions(c...)
}

func (mg *ProviderConfig) GetUsers() int64 {
	return mg.Status.Users
}

func (mg *ProviderConfig) SetUsers(u int64) {
	mg.Status.Users = u
}

// ProviderConfigUsage Methods

func (mg *ProviderConfigUsage) GetProviderConfigReference() xpv1.Reference {
	return mg.ProviderConfigReference
}

func (mg *ProviderConfigUsage) SetProviderConfigReference(r xpv1.Reference) {
	mg.ProviderConfigReference = r
}

func (mg *ProviderConfigUsage) GetResourceReference() xpv1.TypedReference {
	return mg.ResourceReference
}

func (mg *ProviderConfigUsage) SetResourceReference(r xpv1.TypedReference) {
	mg.ResourceReference = r
}
