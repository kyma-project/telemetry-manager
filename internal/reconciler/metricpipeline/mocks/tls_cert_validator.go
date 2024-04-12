// Code generated by mockery v2.42.2. DO NOT EDIT.

package mocks

import (
	context "context"

	mock "github.com/stretchr/testify/mock"

	tlsCert "github.com/kyma-project/telemetry-manager/internal/tlsCert"

	v1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
)

// TLSCertValidator is an autogenerated mock type for the TLSCertValidator type
type TLSCertValidator struct {
	mock.Mock
}

// ResolveAndValidateCertificate provides a mock function with given fields: ctx, certPEM, keyPEM
func (_m *TLSCertValidator) ResolveAndValidateCertificate(ctx context.Context, certPEM *v1alpha1.ValueType, keyPEM *v1alpha1.ValueType) tlsCert.TLSCertValidationResult {
	ret := _m.Called(ctx, certPEM, keyPEM)

	if len(ret) == 0 {
		panic("no return value specified for ResolveAndValidateCertificate")
	}

	var r0 tlsCert.TLSCertValidationResult
	if rf, ok := ret.Get(0).(func(context.Context, *v1alpha1.ValueType, *v1alpha1.ValueType) tlsCert.TLSCertValidationResult); ok {
		r0 = rf(ctx, certPEM, keyPEM)
	} else {
		r0 = ret.Get(0).(tlsCert.TLSCertValidationResult)
	}

	return r0
}

// NewTLSCertValidator creates a new instance of TLSCertValidator. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewTLSCertValidator(t interface {
	mock.TestingT
	Cleanup(func())
}) *TLSCertValidator {
	mock := &TLSCertValidator{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
