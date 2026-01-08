package tlscert

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
)

var (
	// certExpiry is a time when the certificate expires
	certExpiry   = time.Date(2024, time.March, 19, 14, 24, 14, 0, time.UTC)
	pastCaExpiry = time.Date(2023, time.June, 15, 9, 48, 37, 0, time.UTC)
)

var (
	defaultCertData = []byte(`-----BEGIN CERTIFICATE-----
MIICfzCCAeigAwIBAgIUE3joLdLPrpZq45+MXgFPLB1jmaEwDQYJKoZIhvcNAQEL
BQAwXjELMAkGA1UEBhMCVVMxCzAJBgNVBAgMAk5ZMREwDwYDVQQHDAhOZXcgWW9y
azEdMBsGA1UECgwUTW9jayBDQSBPcmdhbml6YXRpb24xEDAOBgNVBAMMB01vY2sg
Q0EwHhcNMjQwMjE5MTQyNDE0WhcNMjQwMzE5MTQyNDE0WjBgMQswCQYDVQQGEwJV
UzELMAkGA1UECAwCTlkxETAPBgNVBAcMCE5ldyBZb3JrMRowGAYDVQQKDBFNb2Nr
IE9yZ2FuaXphdGlvbjEVMBMGA1UEAwwMd3d3Lm1vY2suY29tMIGfMA0GCSqGSIb3
DQEBAQUAA4GNADCBiQKBgQCWI083NIlshIJpH53LJumuxoEzXwWwt9fljCXY+kGV
yomib1KnC74Ofxze8VRE/zpPSJqci6ZWtZd7tJ4Yef9VVB2uVILugPFEgOhv/Ocp
wK1s4JySS6sXQt5fa1N45e/mjzDJt0gJznTJ/nRLtuMapVYZc4MaG/ufJw1Vn7gO
SQIDAQABozgwNjAMBgNVHRMBAf8EAjAAMA4GA1UdDwEB/wQEAwIFoDAWBgNVHSUB
Af8EDDAKBggrBgEFBQcDATANBgkqhkiG9w0BAQsFAAOBgQCE7HjvDRO8/0KAOPjR
5yEb0I4JNNUy+ZwKpSGx4rMW92/p74ibVSNgaBpp9MmHbkyJwdI8z0opBuRuJj5F
ntmGGbBVnit+MAhnFcfmmpUa7rS7r5VWyz4BxBzYqlQC6dO68YBAMLk7MPkDkG0j
vQX+A733KxoD+L2iFmGuutwKUw==
-----END CERTIFICATE-----`)

	defaultKeyData = []byte(`-----BEGIN PRIVATE KEY-----
MIICWwIBAAKBgQCWI083NIlshIJpH53LJumuxoEzXwWwt9fljCXY+kGVyomib1Kn
C74Ofxze8VRE/zpPSJqci6ZWtZd7tJ4Yef9VVB2uVILugPFEgOhv/OcpwK1s4JyS
S6sXQt5fa1N45e/mjzDJt0gJznTJ/nRLtuMapVYZc4MaG/ufJw1Vn7gOSQIDAQAB
AoGATXUMAkwtdfnrGfcAvnVl7BBnSayFUAWY8clbIVUDDxd96HqMZrgNJod3yqEw
u6P9Xjfz5D275FItQ9oMEk6mZoH+GuRYT2nYmYH/Tbv/eXJvydKTLPivbBb+/0Zl
m0Lg+6cU4iThpEBdKOK6F5NrFotmyM6PEgIj6zSdw1LRT/0CQQDHA2o/1yTiEspu
L65PcQu5QKutGfEvxDz12NLAFeeOqrWK85AUkpVOI5y/lL+uLF7gTJVoeOm7gbLo
3JoBqzrXAkEAwSEal9SjPU12/KzHzPb++pywGh2PkR7p8L9EzghOCL6Q8H0WyxbY
pkE4wVCxdrMwtyIm4XsTZD1mtTjfaTn73wJABcEkhloLN/pBHjSEvslPBHlJPYUd
gzsSZC1z0pgPjQGEpFLsnJusc4j2FFgRvtCLocK1I0Mzxvc2HCOc1GWGGwJAZq2N
8OkJPL9hombN9yfeWilR6yCKQrJ32Boon42Ex1thvaoTozfrSUDlxsl7AEu2e7b5
iumfXqzSXUj2ZoCAawJACqdjmy41AqWr++nKRTyuFA4MB2kNdzwi4ZfAJZeeL84R
FUNBWK5CxRp4fxwfEnevbDBBluPZP8Df6E+P60LQZw==
-----END PRIVATE KEY-----`)

	defaultCaData = []byte(`-----BEGIN CERTIFICATE-----
MIICWzCCAcSgAwIBAgIUNUvkfvf5ZxFDlPG0H28bqlCcSjAwDQYJKoZIhvcNAQEL
BQAwXjELMAkGA1UEBhMCVVMxCzAJBgNVBAgMAk5ZMREwDwYDVQQHDAhOZXcgWW9y
azEdMBsGA1UECgwUTW9jayBDQSBPcmdhbml6YXRpb24xEDAOBgNVBAMMB01vY2sg
Q0EwHhcNMjQwNjA0MDUzNDAxWhcNMzQwNjAyMDUzNDAxWjBeMQswCQYDVQQGEwJV
UzELMAkGA1UECAwCTlkxETAPBgNVBAcMCE5ldyBZb3JrMR0wGwYDVQQKDBRNb2Nr
IENBIE9yZ2FuaXphdGlvbjEQMA4GA1UEAwwHTW9jayBDQTCBnzANBgkqhkiG9w0B
AQEFAAOBjQAwgYkCgYEA2WtaVj9idUnw0DiB/Il4M+fMOPcZ35MU6l0R2mrnbQCH
oBDxSDNvqfvnUqomIoLiSSBNbeKft/7/prxfp1ObLSh/uQjo6lzaZMfBmyNWs1Ad
JwlqoOCJna8rcLRII2uX8mNyQQRK49QVtdMkr+dMJqL0Uwa5Hur+XB9KBR+bRNUC
AwEAAaMWMBQwEgYDVR0TAQH/BAgwBgEB/wIBADANBgkqhkiG9w0BAQsFAAOBgQBf
48rRd5wDlX+/0xgQcPLs0igSd87WuLBlLsjX8CNhiO8f6Vh+P+NFxi8LvcA1gKeU
CpCqzRkDXfFEyHPMNVOSTuR10NLaEcAbJo5dIFfbUdX9cViW26XfISydI7zgDuno
WWL1dEpm9rYQvcflxENRpp9SpyG2bJliRexjmHYwFg==
-----END CERTIFICATE-----`)

	pastCaData = []byte(`-----BEGIN CERTIFICATE-----
MIICWzCCAcSgAwIBAgIUee6vIOPHP601JHnmFvyptWNyProwDQYJKoZIhvcNAQEL
BQAwXjELMAkGA1UEBhMCVVMxCzAJBgNVBAgMAk5ZMREwDwYDVQQHDAhOZXcgWW9y
azEdMBsGA1UECgwUTW9jayBDQSBPcmdhbml6YXRpb24xEDAOBgNVBAMMB01vY2sg
Q0EwHhcNMTQwNjE3MDk0ODM3WhcNMjMwNjE1MDk0ODM3WjBeMQswCQYDVQQGEwJV
UzELMAkGA1UECAwCTlkxETAPBgNVBAcMCE5ldyBZb3JrMR0wGwYDVQQKDBRNb2Nr
IENBIE9yZ2FuaXphdGlvbjEQMA4GA1UEAwwHTW9jayBDQTCBnzANBgkqhkiG9w0B
AQEFAAOBjQAwgYkCgYEApZp1OcH38tohYNYnu/MbWhdfg8/oZ70d/UGBw/lDnh+z
eqj6fPSFL0taBiirMUHco+8BlRwNWzwSMnndyBiibLoO/HGItBh2Z7lgvUgETAea
1aPMu15BLeJKbQ9szOYyYsbuZC9X8Hch0QBP25gYJ9PApfTMSWUyoN7XlDbNIwMC
AwEAAaMWMBQwEgYDVR0TAQH/BAgwBgEB/wIBADANBgkqhkiG9w0BAQsFAAOBgQBM
dwv38Fw+YyhjvAbKxf5+208FQDrC2dOJUCDBLE75VxKwj0IXIjxp5cjGsni4GWYy
RtWX6wkDF7yTSEfW0CAfbXCmxp9ln2PNQCF2kB90XPkeXxXum/uAZAIk3GGgxRN8
rZ3xPLf7G+fObmeO7XuIoDfJHH6HDrdhhWi3F918KQ==
-----END CERTIFICATE-----`)
)

func TestMissingCert(t *testing.T) {
	oneMonthBeforeExpiry := pastCaExpiry.Add(-30 * 24 * time.Hour)
	fakeClient := fake.NewClientBuilder().Build()
	validator := Validator{
		client: fakeClient,
		now:    func() time.Time { return oneMonthBeforeExpiry },
	}

	err := validator.Validate(t.Context(), TLSValidationParams{
		Key: &telemetryv1beta1.ValueType{Value: string(defaultKeyData)},
		CA:  &telemetryv1beta1.ValueType{Value: string(defaultCaData)},
	})
	require.ErrorIs(t, err, ErrMissingCertKeyPair)
}

func TestMissingKey(t *testing.T) {
	oneMonthBeforeExpiry := pastCaExpiry.Add(-30 * 24 * time.Hour)
	fakeClient := fake.NewClientBuilder().Build()
	validator := Validator{
		client: fakeClient,
		now:    func() time.Time { return oneMonthBeforeExpiry },
	}

	err := validator.Validate(t.Context(), TLSValidationParams{
		Cert: &telemetryv1beta1.ValueType{Value: string(defaultCertData)},
		CA:   &telemetryv1beta1.ValueType{Value: string(defaultCaData)},
	})
	require.ErrorIs(t, err, ErrMissingCertKeyPair)
}

func TestMissingCA(t *testing.T) {
	oneMonthBeforeExpiry := pastCaExpiry.Add(-30 * 24 * time.Hour)
	fakeClient := fake.NewClientBuilder().Build()
	validator := Validator{
		client: fakeClient,
		now:    func() time.Time { return oneMonthBeforeExpiry },
	}

	err := validator.Validate(t.Context(), TLSValidationParams{
		Cert: &telemetryv1beta1.ValueType{Value: string(defaultCertData)},
		Key:  &telemetryv1beta1.ValueType{Value: string(defaultKeyData)},
	})
	require.NoError(t, err)
}

func TestMissingCertAndKey(t *testing.T) {
	oneMonthBeforeExpiry := pastCaExpiry.Add(-30 * 24 * time.Hour)
	fakeClient := fake.NewClientBuilder().Build()
	validator := Validator{
		client: fakeClient,
		now:    func() time.Time { return oneMonthBeforeExpiry },
	}

	err := validator.Validate(t.Context(), TLSValidationParams{
		CA: &telemetryv1beta1.ValueType{Value: string(defaultCaData)},
	})
	require.NoError(t, err)
}

func TestMissingAll(t *testing.T) {
	oneMonthBeforeExpiry := pastCaExpiry.Add(-30 * 24 * time.Hour)
	fakeClient := fake.NewClientBuilder().Build()
	validator := Validator{
		client: fakeClient,
		now:    func() time.Time { return oneMonthBeforeExpiry },
	}

	err := validator.Validate(t.Context(), TLSValidationParams{})
	require.NoError(t, err)
}

func TestExpiredCertificate(t *testing.T) {
	oneDayAfterExpiry := certExpiry.Add(24 * time.Hour)
	fakeClient := fake.NewClientBuilder().Build()
	validator := Validator{
		client: fakeClient,
		now:    func() time.Time { return oneDayAfterExpiry },
	}

	err := validator.Validate(t.Context(), TLSValidationParams{
		Cert: &telemetryv1beta1.ValueType{Value: string(defaultCertData)},
		Key:  &telemetryv1beta1.ValueType{Value: string(defaultKeyData)},
		CA:   &telemetryv1beta1.ValueType{Value: string(defaultCaData)},
	})
	require.Error(t, err)
	require.True(t, IsCertExpiredError(err))

	var certExpiredErr *CertExpiredError

	require.True(t, errors.As(err, &certExpiredErr))
	require.Equal(t, certExpiry, certExpiredErr.Expiry)
	require.EqualError(t, err, "TLS certificate expired on 2024-03-19")
}

func TestAboutToExpireCertificate(t *testing.T) {
	tests := []struct {
		name        string
		now         time.Time
		expectValid bool
	}{
		{
			name: "expiry day",
			now:  certExpiry,
		},
		{
			name: "one day before expiry",
			now:  certExpiry.Add(-24 * time.Hour),
		},
		{
			name: "two weeks before expiry",
			now:  certExpiry.Add(-twoWeeks),
		},
		{
			name:        "two weeks and one day before expiry",
			now:         certExpiry.Add(-twoWeeks - 24*time.Hour),
			expectValid: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fakeClient := fake.NewClientBuilder().Build()
			validator := Validator{
				client: fakeClient,
				now:    func() time.Time { return test.now },
			}

			err := validator.Validate(t.Context(), TLSValidationParams{
				Cert: &telemetryv1beta1.ValueType{Value: string(defaultCertData)},
				Key:  &telemetryv1beta1.ValueType{Value: string(defaultKeyData)},
				CA:   &telemetryv1beta1.ValueType{Value: string(defaultCaData)},
			})
			if test.expectValid {
				require.NoError(t, err)
				return
			}

			require.Error(t, err)
			require.True(t, IsCertAboutToExpireError(err))

			var certAboutToExpireErr *CertAboutToExpireError

			require.True(t, errors.As(err, &certAboutToExpireErr))
			require.Equal(t, certExpiry, certAboutToExpireErr.Expiry)
			require.EqualError(t, err, "TLS certificate is about to expire, configured certificate is valid until 2024-03-19")
		})
	}
}

func TestValidCertificatesAndPrivateKey(t *testing.T) {
	oneMonthBeforeExpiry := certExpiry.Add(-30 * 24 * time.Hour)
	fakeClient := fake.NewClientBuilder().Build()
	validator := Validator{
		client: fakeClient,
		now:    func() time.Time { return oneMonthBeforeExpiry },
	}

	err := validator.Validate(t.Context(), TLSValidationParams{
		Cert: &telemetryv1beta1.ValueType{Value: string(defaultCertData)},
		Key:  &telemetryv1beta1.ValueType{Value: string(defaultKeyData)},
		CA:   &telemetryv1beta1.ValueType{Value: string(defaultCaData)},
	})
	require.NoError(t, err)
}

func TestInvalidCertificate(t *testing.T) {
	certData := []byte(`-----BEGIN CERTIFICATE-----
MIICNjCCAZ+gAwIBAgIBADANBgkqhkiG9w0BAQ0FADA4MQswCQYDVQQGEwJ1czEL
MAkGA1UECAwCTlkxDTALBgNVBAoMBFRlc3QxDTALBgNVBAMMBFRlc3QwHhcNMjQw
MzIxMTQyNDE0WhcNMjQwMzE5MTQyNDE0WjA4MQswCQYDVQQGEwJ1czELMAkGA1UE
CAwCTlkxDTALBgNVBAoMBFRlc3QxDTALBgNVBAMMBFRlc3QwgZ8wDQYJKoZIhvcN
AQEBBQADgY0AMIGJAoGBAMfSQ/2hwo2Qf5wA5OQ/aFuz/tFbmxwWrxtw1cAG43A9
zG7W75kESVdTiBeKTZRXhiG0+hCa7jKULD5GWczhkwR0wepkJ+LN7SO+XDjT2YX0
hGLfdL8opWn59d/b/0wtE7lz2Q+G/puXlDd85kM9oV+kK8oU74pZ0sNgE5lPd8t9
AgMBAAGjUDBOMB0GA1UdDgQWBBQnFMbU0Hpg5rOfpn66vG6JVp4uXzAfBgNVHSME
GDAWgBQnFMbU0Hpg5rOfpn66vG6JVp4uXzAMBgNVHRMEBTADAQH/MA0GCSqGSIb3
DQEBDQUAA4GBAGl/tj0QW096fknAer/Q2Hmt6KINFjk6tKfnnJYYU22NMp2DQMWB
7mNxmglynPG/0hOw6OpG0ji+yPCPiZ+/RscNWgrCNAUxvsxrT8t0mEPR9lhLmxlV
WxZIBPi0z6MoiZxVKSY8EBeVYCHWS9A2l1J6gAHptihe7y1j8I2ffS
-----END CERTIFICATE-----`)

	fakeClient := fake.NewClientBuilder().Build()
	validator := Validator{
		client: fakeClient,
		now:    func() time.Time { return certExpiry.Add(-24 * time.Hour) },
	}

	err := validator.Validate(t.Context(), TLSValidationParams{
		Cert: &telemetryv1beta1.ValueType{Value: string(certData)},
		Key:  &telemetryv1beta1.ValueType{Value: string(defaultKeyData)},
		CA:   &telemetryv1beta1.ValueType{Value: string(defaultCaData)},
	})
	require.ErrorIs(t, err, ErrCertDecodeFailed)
}

func TestInvalidPrivateKey(t *testing.T) {
	keyData := []byte(`-----BEGIN PRIVATE KEY-----
MIICdgIBADANBgkqhkiG9w0BAQEFAASCAmAwggJcAgEAAoGBAMfSQ/2hwo2Qf5wA
5OQ/aFuz/tFbmxwWrxtw1cAG43A9zG7W75kESVdTiBeKTZRXhiG0+hCa7jKULD5G
WczhkwR0wepkJ+LN7SO+XDjT2YX0hGLfdL8opWn59d/b/0wtE7lz2Q+G/puXlDd8
5kM9oV+kK8oU74pZ0sNgE5lPd8t9AgMBAAECgYB5C3KMbjUAtIvY4OHHMnHxOzQd
drSba1Jf+RZC4Old0NHKQwGZW/NhpwRF3k3okqx6NrtU28V3djLm9o7nga4gbgaj
DIHVVVBBLhPS75aHaaqrol2rL0GuQtymJ9OFjFcnVY4ylU1eOD7Vvdzpgn7VtK47
vvD1uAGypMwma1jOAQJBAPTq13sY+OtHxBSeHRkMyFshjGCc42ES3CclS6i0FiW+
Ns2lQie+VD+chmE0OzkGdRk3IPmzfRyPAGfYBzyWr6ECQQDQ3QZ1KZ+u3kij6CUl
6RgU0fKaiXZT9e0nEC3StlkiaaGfYgyLIEWoGdr3aaiwcFsOlH/1UEuaBY52weHU
kT5dAkEA5ZpPfkBwAypZYTbFcplwLzbpQh1ycKvcpfopzrNdW+7Rs8JsnZOpqaTU
ucXci15JYuUyzcR90sshBzkXt65QYQJAcHbjWEk+c7G7mY6SGjTGQ8e9A5uLPLCK
r2MV2YVYv5/zaFgqeuu4tkid0GVzcPY/Ab3SnOxMmTXuvWGu0YAX/QJAZwN4lwdO
ga5H3f7hUBINasQIdOGEAy3clqCBpLj2eUMXHHNxVsVGBnJOEqckn6fg6pcHnhmK
5VAuzWx+wWwQ==
-----END PRIVATE KEY-----`)

	fakeClient := fake.NewClientBuilder().Build()
	validator := New(fakeClient)

	err := validator.Validate(t.Context(), TLSValidationParams{
		Cert: &telemetryv1beta1.ValueType{Value: string(defaultCertData)},
		Key:  &telemetryv1beta1.ValueType{Value: string(keyData)},
		CA:   &telemetryv1beta1.ValueType{Value: string(defaultCaData)},
	})
	require.ErrorIs(t, err, ErrKeyDecodeFailed)
}

func TestInvalidCA(t *testing.T) {
	caData := []byte(`-----BEGIN CERTIFICATE-----
XMIICWzCCAcSgAwIBAgIUNUvkfvf5ZxFDlPG0H28bqlCcSjAwDQYJKoZIhvcNAQEL
BQAwXjELMAkGA1UEBhMCVVMxCzAJBgNVBAgMAk5ZMREwDwYDVQQHDAhOZXcgWW9y
azEdMBsGA1UECgwUTW9jayBDQSBPcmdhbml6YXRpb24xEDAOBgNVBAMMB01vY2sg
Q0EwHhcNMjQwNjA0MDUzNDAxWhcNMzQwNjAyMDUzNDAxWjBeMQswCQYDVQQGEwJV
UzELMAkGA1UECAwCTlkxETAPBgNVBAcMCE5ldyBZb3JrMR0wGwYDVQQKDBRNb2Nr
IENBIE9yZ2FuaXphdGlvbjEQMA4GA1UEAwwHTW9jayBDQTCBnzANBgkqhkiG9w0B
AQEFAAOBjQAwgYkCgYEA2WtaVj9idUnw0DiB/Il4M+fMOPcZ35MU6l0R2mrnbQCH
oBDxSDNvqfvnUqomIoLiSSBNbeKft/7/prxfp1ObLSh/uQjo6lzaZMfBmyNWs1Ad
JwlqoOCJna8rcLRII2uX8mNyQQRK49QVtdMkr+dMJqL0Uwa5Hur+XB9KBR+bRNUC
AwEAAaMWMBQwEgYDVR0TAQH/BAgwBgEB/wIBADANBgkqhkiG9w0BAQsFAAOBgQBf
48rRd5wDlX+/0xgQcPLs0igSd87WuLBlLsjX8CNhiO8f6Vh+P+NFxi8LvcA1gKeU
CpCqzRkDXfFEyHPMNVOSTuR10NLaEcAbJo5dIFfbUdX9cViW26XfISydI7zgDuno
WWL1dEpm9rYQvcflxENRpp9SpyG2bJliRexjmHYwFg==
-----END CERTIFICATE-----`)

	oneMonthBeforeExpiry := certExpiry.Add(-30 * 24 * time.Hour)
	fakeClient := fake.NewClientBuilder().Build()
	validator := Validator{
		client: fakeClient,
		now:    func() time.Time { return oneMonthBeforeExpiry },
	}

	err := validator.Validate(t.Context(), TLSValidationParams{
		Cert: &telemetryv1beta1.ValueType{Value: string(defaultCertData)},
		Key:  &telemetryv1beta1.ValueType{Value: string(defaultKeyData)},
		CA:   &telemetryv1beta1.ValueType{Value: string(caData)},
	})
	require.ErrorIs(t, err, ErrCADecodeFailed)
}

func TestExpiredCA(t *testing.T) {
	oneMonthBeforeExpiry := certExpiry.Add(-30 * 24 * time.Hour)
	fakeClient := fake.NewClientBuilder().Build()
	validator := Validator{
		client: fakeClient,
		now:    func() time.Time { return oneMonthBeforeExpiry },
	}

	err := validator.Validate(t.Context(), TLSValidationParams{
		Cert: &telemetryv1beta1.ValueType{Value: string(defaultCertData)},
		Key:  &telemetryv1beta1.ValueType{Value: string(defaultKeyData)},
		CA:   &telemetryv1beta1.ValueType{Value: string(pastCaData)},
	})
	require.Error(t, err)
	require.True(t, IsCertExpiredError(err))

	var caExpiredErr *CertExpiredError

	require.True(t, errors.As(err, &caExpiredErr))
	require.Equal(t, pastCaExpiry, caExpiredErr.Expiry)
	require.EqualError(t, err, "TLS CA certificate expired on 2023-06-15")
}

func TestAboutToExpireCA(t *testing.T) {
	tests := []struct {
		name        string
		now         time.Time
		expectValid bool
	}{
		{
			name: "expiry day",
			now:  pastCaExpiry,
		},
		{
			name: "one day before expiry",
			now:  pastCaExpiry.Add(-24 * time.Hour),
		},
		{
			name: "two weeks before expiry",
			now:  pastCaExpiry.Add(-twoWeeks),
		},
		{
			name:        "two weeks and one day before expiry",
			now:         pastCaExpiry.Add(-twoWeeks - 24*time.Hour),
			expectValid: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fakeClient := fake.NewClientBuilder().Build()
			validator := Validator{
				client: fakeClient,
				now:    func() time.Time { return test.now },
			}

			err := validator.Validate(t.Context(), TLSValidationParams{
				Cert: &telemetryv1beta1.ValueType{Value: string(defaultCertData)},
				Key:  &telemetryv1beta1.ValueType{Value: string(defaultKeyData)},
				CA:   &telemetryv1beta1.ValueType{Value: string(pastCaData)},
			})
			if test.expectValid {
				require.NoError(t, err)
				return
			}

			require.Error(t, err)
			require.True(t, IsCertAboutToExpireError(err))

			var caAboutToExpireErr *CertAboutToExpireError

			require.True(t, errors.As(err, &caAboutToExpireErr))
			require.Equal(t, pastCaExpiry, caAboutToExpireErr.Expiry)
			require.EqualError(t, err, "TLS CA certificate is about to expire, configured certificate is valid until 2023-06-15")
		})
	}
}

func TestMultipleCAs(t *testing.T) {
	caData := []byte(`-----BEGIN CERTIFICATE-----
MIICWzCCAcSgAwIBAgIUNUvkfvf5ZxFDlPG0H28bqlCcSjAwDQYJKoZIhvcNAQEL
BQAwXjELMAkGA1UEBhMCVVMxCzAJBgNVBAgMAk5ZMREwDwYDVQQHDAhOZXcgWW9y
azEdMBsGA1UECgwUTW9jayBDQSBPcmdhbml6YXRpb24xEDAOBgNVBAMMB01vY2sg
Q0EwHhcNMjQwNjA0MDUzNDAxWhcNMzQwNjAyMDUzNDAxWjBeMQswCQYDVQQGEwJV
UzELMAkGA1UECAwCTlkxETAPBgNVBAcMCE5ldyBZb3JrMR0wGwYDVQQKDBRNb2Nr
IENBIE9yZ2FuaXphdGlvbjEQMA4GA1UEAwwHTW9jayBDQTCBnzANBgkqhkiG9w0B
AQEFAAOBjQAwgYkCgYEA2WtaVj9idUnw0DiB/Il4M+fMOPcZ35MU6l0R2mrnbQCH
oBDxSDNvqfvnUqomIoLiSSBNbeKft/7/prxfp1ObLSh/uQjo6lzaZMfBmyNWs1Ad
JwlqoOCJna8rcLRII2uX8mNyQQRK49QVtdMkr+dMJqL0Uwa5Hur+XB9KBR+bRNUC
AwEAAaMWMBQwEgYDVR0TAQH/BAgwBgEB/wIBADANBgkqhkiG9w0BAQsFAAOBgQBf
48rRd5wDlX+/0xgQcPLs0igSd87WuLBlLsjX8CNhiO8f6Vh+P+NFxi8LvcA1gKeU
CpCqzRkDXfFEyHPMNVOSTuR10NLaEcAbJo5dIFfbUdX9cViW26XfISydI7zgDuno
WWL1dEpm9rYQvcflxENRpp9SpyG2bJliRexjmHYwFg==
-----END CERTIFICATE-----

-----BEGIN CERTIFICATE-----
MIICWzCCAcSgAwIBAgIUee6vIOPHP601JHnmFvyptWNyProwDQYJKoZIhvcNAQEL
BQAwXjELMAkGA1UEBhMCVVMxCzAJBgNVBAgMAk5ZMREwDwYDVQQHDAhOZXcgWW9y
azEdMBsGA1UECgwUTW9jayBDQSBPcmdhbml6YXRpb24xEDAOBgNVBAMMB01vY2sg
Q0EwHhcNMTQwNjE3MDk0ODM3WhcNMjMwNjE1MDk0ODM3WjBeMQswCQYDVQQGEwJV
UzELMAkGA1UECAwCTlkxETAPBgNVBAcMCE5ldyBZb3JrMR0wGwYDVQQKDBRNb2Nr
IENBIE9yZ2FuaXphdGlvbjEQMA4GA1UEAwwHTW9jayBDQTCBnzANBgkqhkiG9w0B
AQEFAAOBjQAwgYkCgYEApZp1OcH38tohYNYnu/MbWhdfg8/oZ70d/UGBw/lDnh+z
eqj6fPSFL0taBiirMUHco+8BlRwNWzwSMnndyBiibLoO/HGItBh2Z7lgvUgETAea
1aPMu15BLeJKbQ9szOYyYsbuZC9X8Hch0QBP25gYJ9PApfTMSWUyoN7XlDbNIwMC
AwEAAaMWMBQwEgYDVR0TAQH/BAgwBgEB/wIBADANBgkqhkiG9w0BAQsFAAOBgQBM
dwv38Fw+YyhjvAbKxf5+208FQDrC2dOJUCDBLE75VxKwj0IXIjxp5cjGsni4GWYy
RtWX6wkDF7yTSEfW0CAfbXCmxp9ln2PNQCF2kB90XPkeXxXum/uAZAIk3GGgxRN8
rZ3xPLf7G+fObmeO7XuIoDfJHH6HDrdhhWi3F918KQ==
-----END CERTIFICATE-----`)

	oneMonthBeforeExpiry := pastCaExpiry.Add(-30 * 24 * time.Hour)
	fakeClient := fake.NewClientBuilder().Build()
	validator := Validator{
		client: fakeClient,
		now:    func() time.Time { return oneMonthBeforeExpiry },
	}

	err := validator.Validate(t.Context(), TLSValidationParams{
		Cert: &telemetryv1beta1.ValueType{Value: string(defaultCertData)},
		Key:  &telemetryv1beta1.ValueType{Value: string(defaultKeyData)},
		CA:   &telemetryv1beta1.ValueType{Value: string(caData)},
	})
	require.NoError(t, err)
}

func TestEmptyCA(t *testing.T) {
	oneMonthBeforeExpiry := pastCaExpiry.Add(-30 * 24 * time.Hour)
	fakeClient := fake.NewClientBuilder().Build()
	validator := Validator{
		client: fakeClient,
		now:    func() time.Time { return oneMonthBeforeExpiry },
	}

	err := validator.Validate(t.Context(), TLSValidationParams{
		Cert: &telemetryv1beta1.ValueType{Value: string(defaultCertData)},
		Key:  &telemetryv1beta1.ValueType{Value: string(defaultKeyData)},
		CA:   &telemetryv1beta1.ValueType{Value: string([]byte(``))},
	})
	require.ErrorIs(t, err, ErrValueResolveFailed)
}

func TestSanitizeTLSSecretWithEscapedNewLine(t *testing.T) {
	certData := "-----BEGIN CERTIFICATE-----\\nMIICgTCCAeqgAwIBAgIUcejYeiQzytrNkJ9G+V9mxRYZ4rQwDQYJKoZIhvcNAQEL\nBQAwXjELMAkGA1UEBhMCVVMxCzAJBgNVBAgMAk5ZMREwDwYDVQQHDAhOZXcgWW9y\nazEdMBsGA1UECgwUTW9jayBDQSBPcmdhbml6YXRpb24xEDAOBgNVBAMMB01vY2sg\nQ0EwIBcNMjQwMjE5MTQyNDE0WhgPNTAyNDAzMTkxNDI0MTRaMGAxCzAJBgNVBAYT\nAlVTMQswCQYDVQQIDAJOWTERMA8GA1UEBwwITmV3IFlvcmsxGjAYBgNVBAoMEU1v\nY2sgT3JnYW5pemF0aW9uMRUwEwYDVQQDDAx3d3cubW9jay5jb20wgZ8wDQYJKoZI\nhvcNAQEBBQADgY0AMIGJAoGBAN2iJZKzxmxXE8fzw5L45N1xqK1+CTCt5j12fJRT\nzruRxPH5IG1XOiN0vkJUmac3E7o0nsrUaxwP+kb68zZU4mExFVYTI1aVZHQdLXyC\noNBxAtk6Fy9P5fLfRFcovf4t8Frfdn8B4uCwBv5ywV9519kvKUdh0cfW8ZxSXloS\n4hdzAgMBAAGjODA2MAwGA1UdEwEB/wQCMAAwDgYDVR0PAQH/BAQDAgWgMBYGA1Ud\nJQEB/wQMMAoGCCsGAQUFBwMBMA0GCSqGSIb3DQEBCwUAA4GBAJVp4gGV2o1ZHSoR\nrS+Yq4u3eOqA6/nZtKzXq5cSfD4gyzkK/KnFxWZDElMPOrbXUtMVZvoxdyBbmrI8\nPsKzeS+TESxjIQvtIUYxZgbIDePQcXsJSpAjb+QoV0xOonfRtgKTd+s+/KNohDWs\nGWyZ1VBE/Yt3zH4DHhwXChzhUco7\\n-----END CERTIFICATE-----\n"
	keyData := "-----BEGIN PRIVATE KEY-----\\nMIICXQIBAAKBgQDdoiWSs8ZsVxPH88OS+OTdcaitfgkwreY9dnyUU867kcTx+SBt\nVzojdL5CVJmnNxO6NJ7K1GscD/pG+vM2VOJhMRVWEyNWlWR0HS18gqDQcQLZOhcv\nT+Xy30RXKL3+LfBa33Z/AeLgsAb+csFfedfZLylHYdHH1vGcUl5aEuIXcwIDAQAB\nAoGANts0S4w9l4Ex/zKhfJYoJ3tDUbW5VpgkPaA/E4NuztQ0l+OemBGX7UCu+sHv\nygiC1HrDttY+sJJv0vO4EQGPiiLiGzgcuMV96wmRIHw1C3AJvZ9m71/ZoWb9ijIy\nGxe7SBD6Vw2Whyl3K25Wu9ZGAdKFeCbTeY0Dlm7lXw2Ch6kCQQD1QPDxdGjWIaru\n6DX6OwE+IlqwPwb9IdKpKdJzMYLnKmFxfLH5/LxzpKZrxsMZQ+PAFySQWc/ZVXpv\np9QhXLhvAkEA51hAi6D+uaW7EGB7nkNg7bOZjhS3dCkohMcfkQI19uUJWXv8eQPp\nDa4RTM/OqJwJoGKQ+NW5Q+aZwhQwtOirPQJBANMI64tJWQCQ/e4PwIquhTY7B4BK\n66+bkBLiGuXmf7Z8oFawLtFmqZ502oM5CB5QbcSX5W2U6qYfyHgVmRKQH18CQQDa\nrPT2BwxAd4PHCyxOgOoSRf4T60ktp+oA+CfCbhCMfBrGVwhja2rT34HC1XtGrZf7\n3q+iRoOEx2j3pxYTKwsRAkBG4hHXdRZ1a5RnTKi+qeVx5WPtOGuJa1hezGFs6l2p\nstp1Bpl+MQVzbPrhmc2O21g1CRhHDw75iRieVj04dYPP\\n-----END PRIVATE KEY-----\n"
	caData := "-----BEGIN CERTIFICATE-----\\nMIICWzCCAcSgAwIBAgIULINwMAfvhwSrLTiZPMTBHnAso1cwDQYJKoZIhvcNAQEL\nBQAwXjELMAkGA1UEBhMCVVMxCzAJBgNVBAgMAk5ZMREwDwYDVQQHDAhOZXcgWW9y\nazEdMBsGA1UECgwUTW9jayBDQSBPcmdhbml6YXRpb24xEDAOBgNVBAMMB01vY2sg\nQ0EwHhcNMjQwNjA0MTEzNjE5WhcNMzQwNjAyMTEzNjE5WjBeMQswCQYDVQQGEwJV\nUzELMAkGA1UECAwCTlkxETAPBgNVBAcMCE5ldyBZb3JrMR0wGwYDVQQKDBRNb2Nr\nIENBIE9yZ2FuaXphdGlvbjEQMA4GA1UEAwwHTW9jayBDQTCBnzANBgkqhkiG9w0B\nAQEFAAOBjQAwgYkCgYEAvIeSPIhlgbMI18edS09Hkp1jJhtRxLztL0d3PxhoFixK\nSlaoMc1FxGnS0i8keQ4ov46Olqc9+VPEaUIjoYrMS1rx1EsE+wRxXXEULR+vSTsl\nyGiQ+A+KUXXXcOsqT1ZiXyl/xFl4VgSGOiXo1SjexkVvZn8A/NaYDOwSQS/FKLcC\nAwEAAaMWMBQwEgYDVR0TAQH/BAgwBgEB/wIBADANBgkqhkiG9w0BAQsFAAOBgQBj\nitKDY8aqlci8i/HvDJ+/3Cr7NaW7NWxav+V652Fl9ZTikoi/FUFLaegIGKYK0kKP\n0GUn3K0nRZbCe0E5W8RMchf1PGUJCpcBOSNu4/7D/rqynTxHPz4wYoIArJMSq4Gf\nZFnjuqD7zW0+XPmscU7W5VUVC6wGDNr1Xx4OYdTWBQ==\\n-----END CERTIFICATE-----\n"

	fakeClient := fake.NewClientBuilder().Build()
	validator := New(fakeClient)

	err := validator.Validate(t.Context(), TLSValidationParams{
		Cert: &telemetryv1beta1.ValueType{Value: certData},
		Key:  &telemetryv1beta1.ValueType{Value: keyData},
		CA:   &telemetryv1beta1.ValueType{Value: caData},
	})
	require.NoError(t, err)
}

func TestSanitizeValidTLSSecret(t *testing.T) {
	certData := "-----BEGIN CERTIFICATE-----\nMIICgTCCAeqgAwIBAgIUcejYeiQzytrNkJ9G+V9mxRYZ4rQwDQYJKoZIhvcNAQEL\nBQAwXjELMAkGA1UEBhMCVVMxCzAJBgNVBAgMAk5ZMREwDwYDVQQHDAhOZXcgWW9y\nazEdMBsGA1UECgwUTW9jayBDQSBPcmdhbml6YXRpb24xEDAOBgNVBAMMB01vY2sg\nQ0EwIBcNMjQwMjE5MTQyNDE0WhgPNTAyNDAzMTkxNDI0MTRaMGAxCzAJBgNVBAYT\nAlVTMQswCQYDVQQIDAJOWTERMA8GA1UEBwwITmV3IFlvcmsxGjAYBgNVBAoMEU1v\nY2sgT3JnYW5pemF0aW9uMRUwEwYDVQQDDAx3d3cubW9jay5jb20wgZ8wDQYJKoZI\nhvcNAQEBBQADgY0AMIGJAoGBAN2iJZKzxmxXE8fzw5L45N1xqK1+CTCt5j12fJRT\nzruRxPH5IG1XOiN0vkJUmac3E7o0nsrUaxwP+kb68zZU4mExFVYTI1aVZHQdLXyC\noNBxAtk6Fy9P5fLfRFcovf4t8Frfdn8B4uCwBv5ywV9519kvKUdh0cfW8ZxSXloS\n4hdzAgMBAAGjODA2MAwGA1UdEwEB/wQCMAAwDgYDVR0PAQH/BAQDAgWgMBYGA1Ud\nJQEB/wQMMAoGCCsGAQUFBwMBMA0GCSqGSIb3DQEBCwUAA4GBAJVp4gGV2o1ZHSoR\nrS+Yq4u3eOqA6/nZtKzXq5cSfD4gyzkK/KnFxWZDElMPOrbXUtMVZvoxdyBbmrI8\nPsKzeS+TESxjIQvtIUYxZgbIDePQcXsJSpAjb+QoV0xOonfRtgKTd+s+/KNohDWs\nGWyZ1VBE/Yt3zH4DHhwXChzhUco7\n-----END CERTIFICATE-----\n"
	keyData := "-----BEGIN PRIVATE KEY-----\nMIICXQIBAAKBgQDdoiWSs8ZsVxPH88OS+OTdcaitfgkwreY9dnyUU867kcTx+SBt\nVzojdL5CVJmnNxO6NJ7K1GscD/pG+vM2VOJhMRVWEyNWlWR0HS18gqDQcQLZOhcv\nT+Xy30RXKL3+LfBa33Z/AeLgsAb+csFfedfZLylHYdHH1vGcUl5aEuIXcwIDAQAB\nAoGANts0S4w9l4Ex/zKhfJYoJ3tDUbW5VpgkPaA/E4NuztQ0l+OemBGX7UCu+sHv\nygiC1HrDttY+sJJv0vO4EQGPiiLiGzgcuMV96wmRIHw1C3AJvZ9m71/ZoWb9ijIy\nGxe7SBD6Vw2Whyl3K25Wu9ZGAdKFeCbTeY0Dlm7lXw2Ch6kCQQD1QPDxdGjWIaru\n6DX6OwE+IlqwPwb9IdKpKdJzMYLnKmFxfLH5/LxzpKZrxsMZQ+PAFySQWc/ZVXpv\np9QhXLhvAkEA51hAi6D+uaW7EGB7nkNg7bOZjhS3dCkohMcfkQI19uUJWXv8eQPp\nDa4RTM/OqJwJoGKQ+NW5Q+aZwhQwtOirPQJBANMI64tJWQCQ/e4PwIquhTY7B4BK\n66+bkBLiGuXmf7Z8oFawLtFmqZ502oM5CB5QbcSX5W2U6qYfyHgVmRKQH18CQQDa\nrPT2BwxAd4PHCyxOgOoSRf4T60ktp+oA+CfCbhCMfBrGVwhja2rT34HC1XtGrZf7\n3q+iRoOEx2j3pxYTKwsRAkBG4hHXdRZ1a5RnTKi+qeVx5WPtOGuJa1hezGFs6l2p\nstp1Bpl+MQVzbPrhmc2O21g1CRhHDw75iRieVj04dYPP\n-----END PRIVATE KEY-----\n"
	caData := "-----BEGIN CERTIFICATE-----\nMIICWzCCAcSgAwIBAgIULINwMAfvhwSrLTiZPMTBHnAso1cwDQYJKoZIhvcNAQEL\nBQAwXjELMAkGA1UEBhMCVVMxCzAJBgNVBAgMAk5ZMREwDwYDVQQHDAhOZXcgWW9y\nazEdMBsGA1UECgwUTW9jayBDQSBPcmdhbml6YXRpb24xEDAOBgNVBAMMB01vY2sg\nQ0EwHhcNMjQwNjA0MTEzNjE5WhcNMzQwNjAyMTEzNjE5WjBeMQswCQYDVQQGEwJV\nUzELMAkGA1UECAwCTlkxETAPBgNVBAcMCE5ldyBZb3JrMR0wGwYDVQQKDBRNb2Nr\nIENBIE9yZ2FuaXphdGlvbjEQMA4GA1UEAwwHTW9jayBDQTCBnzANBgkqhkiG9w0B\nAQEFAAOBjQAwgYkCgYEAvIeSPIhlgbMI18edS09Hkp1jJhtRxLztL0d3PxhoFixK\nSlaoMc1FxGnS0i8keQ4ov46Olqc9+VPEaUIjoYrMS1rx1EsE+wRxXXEULR+vSTsl\nyGiQ+A+KUXXXcOsqT1ZiXyl/xFl4VgSGOiXo1SjexkVvZn8A/NaYDOwSQS/FKLcC\nAwEAAaMWMBQwEgYDVR0TAQH/BAgwBgEB/wIBADANBgkqhkiG9w0BAQsFAAOBgQBj\nitKDY8aqlci8i/HvDJ+/3Cr7NaW7NWxav+V652Fl9ZTikoi/FUFLaegIGKYK0kKP\n0GUn3K0nRZbCe0E5W8RMchf1PGUJCpcBOSNu4/7D/rqynTxHPz4wYoIArJMSq4Gf\nZFnjuqD7zW0+XPmscU7W5VUVC6wGDNr1Xx4OYdTWBQ==\n-----END CERTIFICATE-----\n"

	fakeClient := fake.NewClientBuilder().Build()
	validator := New(fakeClient)

	err := validator.Validate(t.Context(), TLSValidationParams{
		Cert: &telemetryv1beta1.ValueType{Value: certData},
		Key:  &telemetryv1beta1.ValueType{Value: keyData},
		CA:   &telemetryv1beta1.ValueType{Value: caData},
	})
	require.NoError(t, err)
}

func TestResolveValue(t *testing.T) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"cert": []byte("cert"),
			"key":  []byte("key"),
			"ca":   []byte("ca"),
		},
	}

	fakeClient := fake.NewClientBuilder().WithObjects(secret).Build()
	validator := New(fakeClient)

	tests := []struct {
		name        string
		inputCert   telemetryv1beta1.ValueType
		inputKey    telemetryv1beta1.ValueType
		inputCa     telemetryv1beta1.ValueType
		expectedErr error
	}{
		{
			name: "cert missing",
			inputCert: telemetryv1beta1.ValueType{ValueFrom: &telemetryv1beta1.ValueFromSource{
				SecretKeyRef: &telemetryv1beta1.SecretKeyRef{
					Name:      "unknown",
					Namespace: "default",
					Key:       "cert",
				}},
			},
			inputKey: telemetryv1beta1.ValueType{ValueFrom: &telemetryv1beta1.ValueFromSource{
				SecretKeyRef: &telemetryv1beta1.SecretKeyRef{
					Name:      "test",
					Namespace: "default",
					Key:       "key",
				}},
			},
			inputCa: telemetryv1beta1.ValueType{ValueFrom: &telemetryv1beta1.ValueFromSource{
				SecretKeyRef: &telemetryv1beta1.SecretKeyRef{
					Name:      "test",
					Namespace: "default",
					Key:       "ca",
				}},
			},
			expectedErr: ErrValueResolveFailed,
		},
		{
			name: "key missing",
			inputCert: telemetryv1beta1.ValueType{ValueFrom: &telemetryv1beta1.ValueFromSource{
				SecretKeyRef: &telemetryv1beta1.SecretKeyRef{
					Name:      "test",
					Namespace: "default",
					Key:       "cert",
				}},
			},
			inputKey: telemetryv1beta1.ValueType{ValueFrom: &telemetryv1beta1.ValueFromSource{
				SecretKeyRef: &telemetryv1beta1.SecretKeyRef{
					Name:      "unknown",
					Namespace: "default",
					Key:       "key",
				}},
			},
			inputCa: telemetryv1beta1.ValueType{ValueFrom: &telemetryv1beta1.ValueFromSource{
				SecretKeyRef: &telemetryv1beta1.SecretKeyRef{
					Name:      "test",
					Namespace: "default",
					Key:       "ca",
				}},
			},
			expectedErr: ErrValueResolveFailed,
		},
		{
			name: "ca missing",
			inputCert: telemetryv1beta1.ValueType{ValueFrom: &telemetryv1beta1.ValueFromSource{
				SecretKeyRef: &telemetryv1beta1.SecretKeyRef{
					Name:      "test",
					Namespace: "default",
					Key:       "cert",
				}},
			},
			inputKey: telemetryv1beta1.ValueType{ValueFrom: &telemetryv1beta1.ValueFromSource{
				SecretKeyRef: &telemetryv1beta1.SecretKeyRef{
					Name:      "test",
					Namespace: "default",
					Key:       "key",
				}},
			},
			inputCa: telemetryv1beta1.ValueType{ValueFrom: &telemetryv1beta1.ValueFromSource{
				SecretKeyRef: &telemetryv1beta1.SecretKeyRef{
					Name:      "unknown",
					Namespace: "default",
					Key:       "ca",
				}},
			},
			expectedErr: ErrValueResolveFailed,
		},
		{
			name: "ca empty",
			inputCert: telemetryv1beta1.ValueType{ValueFrom: &telemetryv1beta1.ValueFromSource{
				SecretKeyRef: &telemetryv1beta1.SecretKeyRef{
					Name:      "test",
					Namespace: "default",
					Key:       "cert",
				}},
			},
			inputKey: telemetryv1beta1.ValueType{ValueFrom: &telemetryv1beta1.ValueFromSource{
				SecretKeyRef: &telemetryv1beta1.SecretKeyRef{
					Name:      "test",
					Namespace: "default",
					Key:       "key",
				}},
			},
			inputCa:     telemetryv1beta1.ValueType{Value: ""},
			expectedErr: ErrValueResolveFailed,
		},
		{
			name: "certs and key are present",
			inputCert: telemetryv1beta1.ValueType{ValueFrom: &telemetryv1beta1.ValueFromSource{
				SecretKeyRef: &telemetryv1beta1.SecretKeyRef{
					Name:      "test",
					Namespace: "default",
					Key:       "cert",
				}},
			},
			inputKey: telemetryv1beta1.ValueType{ValueFrom: &telemetryv1beta1.ValueFromSource{
				SecretKeyRef: &telemetryv1beta1.SecretKeyRef{
					Name:      "test",
					Namespace: "default",
					Key:       "key",
				}},
			},
			inputCa: telemetryv1beta1.ValueType{ValueFrom: &telemetryv1beta1.ValueFromSource{
				SecretKeyRef: &telemetryv1beta1.SecretKeyRef{
					Name:      "test",
					Namespace: "default",
					Key:       "ca",
				}},
			},
			expectedErr: ErrCertDecodeFailed,
		},
		{
			name:        "secret is not set",
			inputCert:   telemetryv1beta1.ValueType{ValueFrom: nil},
			inputKey:    telemetryv1beta1.ValueType{ValueFrom: nil},
			inputCa:     telemetryv1beta1.ValueType{ValueFrom: nil},
			expectedErr: ErrValueResolveFailed,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tt := test
			tlsConfig := TLSValidationParams{
				Cert: &tt.inputCert,
				Key:  &tt.inputKey,
				CA:   &tt.inputCa,
			}
			err := validator.Validate(t.Context(), tlsConfig)
			require.ErrorIs(t, err, tt.expectedErr)
		})
	}
}

func TestInvalidCertificateKeyPair(t *testing.T) {
	_, clientCertsFoo, err := testutils.NewCertBuilder("foo", "fooNs").Build()
	require.NoError(t, err)
	serverCertsBar, clientCertsBar, err := testutils.NewCertBuilder("bar", "barNs").Build()
	require.NoError(t, err)

	keyData := clientCertsFoo.ClientKeyPem.String()
	certData := clientCertsBar.ClientCertPem.String()
	caData := serverCertsBar.ServerCertPem.String()

	fakeClient := fake.NewClientBuilder().Build()
	validator := New(fakeClient)

	err = validator.Validate(t.Context(), TLSValidationParams{
		Cert: &telemetryv1beta1.ValueType{Value: certData},
		Key:  &telemetryv1beta1.ValueType{Value: keyData},
		CA:   &telemetryv1beta1.ValueType{Value: caData},
	})
	require.ErrorIs(t, err, ErrInvalidCertificateKeyPair)
}

// It should check first if the certificate key pair match before testing if cert is expired
func TestInvalidCertPair_WithExpiredCert(t *testing.T) {
	_, clientCertsFoo, err := testutils.NewCertBuilder("foo", "fooNs").WithExpiredClientCert().Build()
	require.NoError(t, err)
	serverCertsBar, clientCertsBar, err := testutils.NewCertBuilder("bar", "barNs").Build()
	require.NoError(t, err)

	keyData := clientCertsFoo.ClientKeyPem.String()
	certData := clientCertsBar.ClientCertPem.String()
	caData := serverCertsBar.ServerCertPem.String()

	fakeClient := fake.NewClientBuilder().Build()
	validator := New(fakeClient)

	err = validator.Validate(t.Context(), TLSValidationParams{
		Cert: &telemetryv1beta1.ValueType{Value: certData},
		Key:  &telemetryv1beta1.ValueType{Value: keyData},
		CA:   &telemetryv1beta1.ValueType{Value: caData},
	})
	require.ErrorIs(t, err, ErrInvalidCertificateKeyPair)
}

// Not a CA certificate (CA:FALSE)
func TestInvalidCANotCA(t *testing.T) {
	certData := []byte(`-----BEGIN CERTIFICATE-----
MIICgTCCAeqgAwIBAgIUeNlrXejD+szy492fj5814VxJH2YwDQYJKoZIhvcNAQEL
BQAwXjELMAkGA1UEBhMCVVMxCzAJBgNVBAgMAk5ZMREwDwYDVQQHDAhOZXcgWW9y
azEdMBsGA1UECgwUTW9jayBDQSBPcmdhbml6YXRpb24xEDAOBgNVBAMMB01vY2sg
Q0EwIBcNMjQwMjE5MTQyNDE0WhgPNTAyNDAzMTkxNDI0MTRaMGAxCzAJBgNVBAYT
AlVTMQswCQYDVQQIDAJOWTERMA8GA1UEBwwITmV3IFlvcmsxGjAYBgNVBAoMEU1v
Y2sgT3JnYW5pemF0aW9uMRUwEwYDVQQDDAx3d3cubW9jay5jb20wgZ8wDQYJKoZI
hvcNAQEBBQADgY0AMIGJAoGBAKdNzahbpV6yD3j2D1byhquftHxCUiPPxWYCCyiG
wC0ExGhQEfFu+1jVCNDcLD2RHEdAt1XHIbU7TLzE2kg1Nv0UjCAf7x9LWEMTfb4U
D3anKXBaNpKm5Tfw3DHj0ndb8bGCR4NB/73vCdi857y9tqVLQIF67daXZOw6nq+K
a4FLAgMBAAGjODA2MAwGA1UdEwEB/wQCMAAwDgYDVR0PAQH/BAQDAgWgMBYGA1Ud
JQEB/wQMMAoGCCsGAQUFBwMBMA0GCSqGSIb3DQEBCwUAA4GBACgzPPPSVTkDt2PX
nYHpDMpFmQ8TwyhvA+v75FRAtX714Ge+gIJBbTKtsya8EiZ5LjxZQ8/0rgCKLDNw
fJrJhoy/uBZwRsW5buVmh8H0ehNZh+rLJ1rJAHhV68/8AK5Kw7uqZAC+APbxkXlk
su4DZd4XGoN9rhYKw/S1A5YPe0xT
-----END CERTIFICATE-----`)

	keyData := []byte(`-----BEGIN PRIVATE KEY-----
MIICXAIBAAKBgQCnTc2oW6Vesg949g9W8oarn7R8QlIjz8VmAgsohsAtBMRoUBHx
bvtY1QjQ3Cw9kRxHQLdVxyG1O0y8xNpINTb9FIwgH+8fS1hDE32+FA92pylwWjaS
puU38Nwx49J3W/GxgkeDQf+97wnYvOe8vbalS0CBeu3Wl2TsOp6vimuBSwIDAQAB
AoGBAItm0bi5fCZWOXwxkoCBHmM8dDehTy3VvoYLp96BwPkB4uGD7h98uOPAxlK5
UgeOtMBOFTTc8qI+oeccI0FNTd3jMid3SZEmcQQqntxqmRsKOsjzXWzB3gHlfzIH
xo33HcasAan1ke7gZOIs5FaSspDUbraxMA8NiyNvukjyhJ9JAkEA1vrAnjf4hZ8z
MM6zvbNmtLVogovEoKX74oOEhGnomV98P/eEH51rmeU8MSz9q/UR7T3JLgg0inVg
NlaKKdl7BQJBAMc6NbmLkcDjhND9rqCvpHZ1wno6YVcR6pgfdQqSIxr4V77MFtwB
9iLQNI6DA0G81WTF3lglENhNWtNh1CEx3A8CQGeJ5XNOabeRcUo0g4T9/p1SMb+O
KWlmB+aUiSJtD8Wuo1z7jPrdCpHYQiE9Ff/XzIaCl35AHW4CEhCZpXl6cTECQEXF
tRsOLjWHePRYY9gSq15xT3LPD1gXBjnQioTxOSow30oK39adOT5n/IAMkg9rurBY
O85S7NtT/AMbt9cIRzECQFWWtCLvvav8kabghIiLvn1Bu2htYkzLxKv+QINnjhAb
MinPOrsGkg6m+x67cCAn86xZXlLtnBtv67IBD+EGvVk=
-----END PRIVATE KEY-----
`)

	caData := []byte(`-----BEGIN CERTIFICATE-----
MIICVTCCAb6gAwIBAgIUBF4X2hkHbfc2H4EoBX4JSbisOGMwDQYJKoZIhvcNAQEL
BQAwXjELMAkGA1UEBhMCVVMxCzAJBgNVBAgMAk5ZMREwDwYDVQQHDAhOZXcgWW9y
azEdMBsGA1UECgwUTW9jayBDQSBPcmdhbml6YXRpb24xEDAOBgNVBAMMB01vY2sg
Q0EwHhcNMjQwNjA0MTQzNTA2WhcNMzQwNjAyMTQzNTA2WjBeMQswCQYDVQQGEwJV
UzELMAkGA1UECAwCTlkxETAPBgNVBAcMCE5ldyBZb3JrMR0wGwYDVQQKDBRNb2Nr
IENBIE9yZ2FuaXphdGlvbjEQMA4GA1UEAwwHTW9jayBDQTCBnzANBgkqhkiG9w0B
AQEFAAOBjQAwgYkCgYEAyKCctw2mi6hdghpnujBF0eYrWjhyCN/vDKu9NuifCO4U
GjDee0UONGLpHDMsU9YHb1xJbiDIdL3W038Kb3qkNnk8rp8HBUm2PZ3Rrns5N+F9
/swXb75rs4LgQMiy03REFy2kw1W1HKiFM7qHegw3UIW/ueBNR78aj5yQGeABBukC
AwEAAaMQMA4wDAYDVR0TAQH/BAIwADANBgkqhkiG9w0BAQsFAAOBgQDGXK3Zc0gI
NYlBVHgsasa340ShaO48iVhdJqHz7B4zvbFdI3uG9qL61Op+faw8NIbeYS/Ao3yF
MhlZnoJIG5seAbmbG4U0fAZAMeInGSki4tuEh3VG8aq8TtnbV4FPir7h8bz1AeBO
hhEW5poLfUe8MIvCQoO1GrDpnNZOn7tMjg==
-----END CERTIFICATE-----`)

	oneMonthBeforeExpiry := certExpiry.Add(-30 * 24 * time.Hour)
	fakeClient := fake.NewClientBuilder().Build()
	validator := Validator{
		client: fakeClient,
		now:    func() time.Time { return oneMonthBeforeExpiry },
	}

	err := validator.Validate(t.Context(), TLSValidationParams{
		Cert: &telemetryv1beta1.ValueType{Value: string(certData)},
		Key:  &telemetryv1beta1.ValueType{Value: string(keyData)},
		CA:   &telemetryv1beta1.ValueType{Value: string(caData)},
	})
	require.ErrorIs(t, err, ErrCertIsNotCA)
}

func TestECPrivateKeyAndCertificate(t *testing.T) {
	certData := []byte(`-----BEGIN CERTIFICATE-----
MIICRDCCAemgAwIBAgIUSKl8F4FByWmop14MZSzZPLWRAgYwCgYIKoZIzj0EAwIw
dzELMAkGA1UEBhMCREUxDzANBgNVBAgMBk11bmljaDEPMA0GA1UEBwwGTXVuaWNo
MQwwCgYDVQQKDANTQVAxDDAKBgNVBAsMA0JUUDENMAsGA1UEAwwEa3ltYTEbMBkG
CSqGSIb3DQEJARYMa3ltYUBzYXAuY29tMB4XDTI1MDMxMzExMDAxOVoXDTI2MDMw
ODExMDAxOVowdzELMAkGA1UEBhMCREUxDzANBgNVBAgMBk11bmljaDEPMA0GA1UE
BwwGTXVuaWNoMQwwCgYDVQQKDANTQVAxDDAKBgNVBAsMA0JUUDENMAsGA1UEAwwE
a3ltYTEbMBkGCSqGSIb3DQEJARYMa3ltYUBzYXAuY29tMFkwEwYHKoZIzj0CAQYI
KoZIzj0DAQcDQgAE+B224FGFaVlXmgFGmHY2VgcwZsDrMZ5PHDEbk/qotP7gXvnE
RLYkin/teDa5g0ku0oO9LfLphbvaTUDhscko4qNTMFEwHQYDVR0OBBYEFHU8Ljhf
/eN55R67e6V2kuB5nm+TMB8GA1UdIwQYMBaAFHU8Ljhf/eN55R67e6V2kuB5nm+T
MA8GA1UdEwEB/wQFMAMBAf8wCgYIKoZIzj0EAwIDSQAwRgIhAKrG1bZHjZCbdRdz
OoMLUU2Vjqaue2KTBw00LeNT/Cj3AiEAnnsJqLLhYpje+sk/5G/hCEbYcJnkotLO
k5fBmk2DBx8=
-----END CERTIFICATE-----`)

	keyData := []byte(`-----BEGIN EC PRIVATE KEY-----
MHcCAQEEIL9ckfwWzOlyNQg1VfKrnD62tNj+R0zjMqszlgKpv4ncoAoGCCqGSM49
AwEHoUQDQgAE+B224FGFaVlXmgFGmHY2VgcwZsDrMZ5PHDEbk/qotP7gXvnERLYk
in/teDa5g0ku0oO9LfLphbvaTUDhscko4g==
-----END EC PRIVATE KEY-----
`)

	oneMonthBeforeExpiry := certExpiry.Add(-30 * 24 * time.Hour)
	fakeClient := fake.NewClientBuilder().Build()
	validator := Validator{
		client: fakeClient,
		now:    func() time.Time { return oneMonthBeforeExpiry },
	}

	err := validator.Validate(t.Context(), TLSValidationParams{
		Cert: &telemetryv1beta1.ValueType{Value: string(certData)},
		Key:  &telemetryv1beta1.ValueType{Value: string(keyData)},
	})
	require.NoError(t, err)
}
