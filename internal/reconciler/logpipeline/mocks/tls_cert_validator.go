// Code generated by mockery v2.21.3. DO NOT EDIT.

package mocks

import (
	context "context"

	mock "github.com/stretchr/testify/mock"

	v1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
)

// TLSCertValidator is an autogenerated mock type for the TLSCertValidator type
type TLSCertValidator struct {
	mock.Mock
}

// ValidateCertificate provides a mock function with given fields: ctx, cert, key
func (_m *TLSCertValidator) ValidateCertificate(ctx context.Context, cert *v1alpha1.ValueType, key *v1alpha1.ValueType) error {
	ret := _m.Called(ctx, cert, key)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *v1alpha1.ValueType, *v1alpha1.ValueType) error); ok {
		r0 = rf(ctx, cert, key)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

type mockConstructorTestingTNewTLSCertValidator interface {
	mock.TestingT
	Cleanup(func())
}

// NewTLSCertValidator creates a new instance of TLSCertValidator. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewTLSCertValidator(t mockConstructorTestingTNewTLSCertValidator) *TLSCertValidator {
	mock := &TLSCertValidator{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
