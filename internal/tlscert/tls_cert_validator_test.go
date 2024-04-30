package tlscert

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/testutils"
)

var (
	// certExpiry is a time when the certificate expires
	certExpiry = time.Date(2024, time.March, 19, 14, 24, 14, 0, time.UTC)
)

func TestExpiredCertificate(t *testing.T) {
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
WxZIBPi0z6MoiZxVKSY8EBeVYCHWS9A2l1J6gAHptihe7y1j8I2ffSHm
-----END CERTIFICATE-----`)

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
5VAuzWx+wV5WwQ==
-----END PRIVATE KEY-----`)

	oneDayAfterExpiry := certExpiry.Add(24 * time.Hour)
	fakeClient := fake.NewClientBuilder().Build()
	validator := Validator{
		client: fakeClient,
		now:    func() time.Time { return oneDayAfterExpiry },
	}

	cert := telemetryv1alpha1.ValueType{
		Value: string(certData),
	}

	key := telemetryv1alpha1.ValueType{
		Value: string(keyData),
	}

	err := validator.ValidateCertificate(context.Background(), &cert, &key)
	require.Error(t, err)
	require.True(t, IsCertExpiredError(err))

	var certExpiredErr *CertExpiredError
	require.True(t, errors.As(err, &certExpiredErr))
	require.Equal(t, certExpiry, certExpiredErr.Expiry)
	require.EqualError(t, err, "cert expired on 2024-03-19 14:24:14 +0000 UTC")
}

func TestAboutToExpireCertificate(t *testing.T) {
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
WxZIBPi0z6MoiZxVKSY8EBeVYCHWS9A2l1J6gAHptihe7y1j8I2ffSHm
-----END CERTIFICATE-----`)

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
5VAuzWx+wV5WwQ==
-----END PRIVATE KEY-----`)

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

			cert := telemetryv1alpha1.ValueType{
				Value: string(certData),
			}

			key := telemetryv1alpha1.ValueType{
				Value: string(keyData),
			}

			err := validator.ValidateCertificate(context.Background(), &cert, &key)
			if test.expectValid {
				require.NoError(t, err)
				return
			}

			require.Error(t, err)
			require.True(t, IsCertAboutToExpireError(err))

			var certAboutToExpireErr *CertAboutToExpireError
			require.True(t, errors.As(err, &certAboutToExpireErr))
			require.Equal(t, certExpiry, certAboutToExpireErr.Expiry)
			require.EqualError(t, err, "cert is about to expire, it is valid until 2024-03-19 14:24:14 +0000 UTC")
		})
	}
}

func TestValidCertificateAndPrivateKey(t *testing.T) {

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
WxZIBPi0z6MoiZxVKSY8EBeVYCHWS9A2l1J6gAHptihe7y1j8I2ffSHm
-----END CERTIFICATE-----`)

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
5VAuzWx+wV5WwQ==
-----END PRIVATE KEY-----`)

	oneMonthBeforeExpiry := certExpiry.Add(-30 * 24 * time.Hour)
	fakeClient := fake.NewClientBuilder().Build()
	validator := Validator{
		client: fakeClient,
		now:    func() time.Time { return oneMonthBeforeExpiry },
	}

	cert := telemetryv1alpha1.ValueType{
		Value: string(certData),
	}

	key := telemetryv1alpha1.ValueType{
		Value: string(keyData),
	}

	err := validator.ValidateCertificate(context.Background(), &cert, &key)
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
5VAuzWx+wV5WwQ==
-----END PRIVATE KEY-----`)
	fakeClient := fake.NewClientBuilder().Build()
	validator := Validator{
		client: fakeClient,
		now:    func() time.Time { return certExpiry.Add(-24 * time.Hour) },
	}

	cert := telemetryv1alpha1.ValueType{
		Value: string(certData),
	}

	key := telemetryv1alpha1.ValueType{
		Value: string(keyData),
	}

	err := validator.ValidateCertificate(context.Background(), &cert, &key)
	require.ErrorIs(t, err, ErrCertDecodeFailed)
}

func TestInvalidPrivateKey(t *testing.T) {

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
WxZIBPi0z6MoiZxVKSY8EBeVYCHWS9A2l1J6gAHptihe7y1j8I2ffSHm
-----END CERTIFICATE-----`)

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

	cert := telemetryv1alpha1.ValueType{
		Value: string(certData),
	}

	key := telemetryv1alpha1.ValueType{
		Value: string(keyData),
	}

	err := validator.ValidateCertificate(context.Background(), &cert, &key)
	require.ErrorIs(t, err, ErrKeyDecodeFailed)
}

func TestSanitizeTLSSecretWithEscapedNewLine(t *testing.T) {
	keyData := "-----BEGIN PRIVATE KEY-----\\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCu9t8rBBx0VlnA\nrkIqmjUcdh+nqFje5w4hNCWAxwZYhmLioldyTVGTAoZ/ivP/q2sqKgoLIKu+9LYu\n1k762T5xuhWUfW+BD4mVQuoF6NxJlOW1UgttThXlDD0/MWjQ4CL4+iLhlr7OK3Pp\naQwlak0nlgO4U6eK2vTjkTbgZYXRnRusW7OSfYDlcUSqU7TrmwJuXvug6QobmPWY\nSdqaTG7cLqF+5Voy1w2JKhceQ1DGvUZ8qVFNuUZivytdEsGpnxaTEURK0scgroYX\npP/j1W9CV2eJYGI/YMGrtuMz0SRTP7zh/ZyUs3xtUcpnEkASuELd7GKWQ+JliLnH\nlTqBh/LfAgMBAAECggEAF7pWMKIUQ84UH6E3oEnH1c4f49+yUbnEZ59dTgDFF7X9\n7Mi3Eu1xGH6kA+GxznoOWf+tMK4jweKztFgPCk5HqCpkKSj2ZBUvLkVkBxzPMdFr\nckgn/Du16hlbUwRzN7ngiaMj84kQjlWZQ3h3aQ7osMDTof4CVOlLLcjbSC+sfW0s\nJacmcgP1rYVAcBo2IAo60WAxZBQzKbSHRSVS0g9khUoinfcdKDeyYGze//AiXXXR\nriJ9AYjGTsafV9+VCk6crcznk66eeJXAatecy4Jeu0k5vTta2GAeVkTviNQ4WlQA\nJIFd9uRAUAO5opra/MG9aDi62XcpvDTq8h6Clk5zqQKBgQD27KUIOrIlOnfeosGz\nIjNTMHXrDFG04t05jClSO+6D7mBr7l+t5rS7tPIsKtyibVcYggZKFa8jIYl30dxE\nL3/A8WVRRTZ6AckznzhyIbF1imTzK1sDbNSWwHvoICQ71DPO4nWwD/HnF5owgq9o\npAD/1vnAJLOgO9b/MsN4kip2twKBgQC1ZSRpjRkLgPO5uNxWVjSCf1s0fY9o6Byi\ngNmpsBwQ4by3+VOL+G0JrX1puZV/QL4vNZZ4fNo8xK9b16FmvWwImqadVr6AOzU7\nfMSHo5DUv+mXjA0KHBkdqbWLg3ppGicGYohnotq2lx+2Kubl039OdB+KtXF2OVgx\nnX2PKO19GQKBgGw1aF0i287UwJMgYCJQao2aPxKyY1wRz0DY24LeILhQTpD99ZAP\n+kQIF9ijL+0+XVywHnF47zdGCygnH5ACAMpc/zmOS0FMZw/oRqQ9f7cy3upxpYDq\nwH8P+zzOWRKe+9U+CLUPR8Mt5LQ9kQEaXhW/79L0QoOFtcJATMkZxOIhAoGBAIAe\ntQ48U5E1fnASKsZsUuBNNc0oVi+Bqh/5JEPfGKOv3UyQNLtrNxCb0jXnl7jusKXF\nksb9YGOFhFo5Pk3Dwtd86+u7hggqSZn/sQwgsj4iYsngaKFYYUD7SjgFIGO1zhSL\nac7RTuuiaAqR2M5BiOyPxmuBZmdbb3hzxWhlPwCZAoGAfYn3JgsVTp+DnXHOU9j5\nt1tKHLafi7nRVXmnHbFABmLC0KyJfWZqaCdUoJHRNhggAu918BWA2hlEWjmIG+id\nWiJ4y5oa63TrG/cr9zvdNHBy30KzvpK+aTgV5uPni7iV7URC1EEhghn6stB3LuvB\n0oPRUIeUnfrscWwxggtUGFE=\\n-----END PRIVATE KEY-----\n"
	certData := "-----BEGIN CERTIFICATE-----\\nMIIDLzCCAhegAwIBAgIUTDC2e9uCi0ggzWiI7XkkXNWmMY0wDQYJKoZIhvcNAQEL\nBQAwJzELMAkGA1UEBhMCVVMxGDAWBgNVBAMMD0V4YW1wbGUtZm9vLWJhcjAeFw0y\nMzEyMjgxMzUwNDNaFw0yNjEwMTcxMzUwNDNaMCcxCzAJBgNVBAYTAlVTMRgwFgYD\nVQQDDA9FeGFtcGxlLWZvby1iYXIwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEK\nAoIBAQCu9t8rBBx0VlnArkIqmjUcdh+nqFje5w4hNCWAxwZYhmLioldyTVGTAoZ/\nivP/q2sqKgoLIKu+9LYu1k762T5xuhWUfW+BD4mVQuoF6NxJlOW1UgttThXlDD0/\nMWjQ4CL4+iLhlr7OK3PpaQwlak0nlgO4U6eK2vTjkTbgZYXRnRusW7OSfYDlcUSq\nU7TrmwJuXvug6QobmPWYSdqaTG7cLqF+5Voy1w2JKhceQ1DGvUZ8qVFNuUZivytd\nEsGpnxaTEURK0scgroYXpP/j1W9CV2eJYGI/YMGrtuMz0SRTP7zh/ZyUs3xtUcpn\nEkASuELd7GKWQ+JliLnHlTqBh/LfAgMBAAGjUzBRMB0GA1UdDgQWBBTYsQEqc5CX\nzjBGv/O04Qd5sOu5QjAfBgNVHSMEGDAWgBTYsQEqc5CXzjBGv/O04Qd5sOu5QjAP\nBgNVHRMBAf8EBTADAQH/MA0GCSqGSIb3DQEBCwUAA4IBAQA1L7IsQ9GTFl9GQMGo\n+JOffZxhR9AiwnCPXTMWF8qYC99F39i946wJIkgJN6wm8Rt46gA65EfJw6YdjdB6\n8kjg3CDRDIFn2QRrP4x8tS4EBu9tUkss/2h0I16MEEB9RV8adjH0lkiPwQwP50uW\nwLlwMHw9KsxA1MATzSmBruzW//gyoJFaBKYsYqYa7VKcEyQqKgiQypBN2O01twF3\ntahLFTIeeD0e4fMe++mwJh8rT5sRpCLmFDIoajmLkjj48P7hvgtLFN+vRTqgqViq\nySngIMt75xyXeTm15o7LrEe4B4HkpWt4CbeUW/44HrCUoItuhyea7baGecLx8VoS\nR3xg\\n-----END CERTIFICATE-----\n"

	fakeClient := fake.NewClientBuilder().Build()
	validator := New(fakeClient)

	cert := telemetryv1alpha1.ValueType{
		Value: certData,
	}

	key := telemetryv1alpha1.ValueType{
		Value: keyData,
	}

	err := validator.ValidateCertificate(context.Background(), &cert, &key)
	require.NoError(t, err)
}

func TestSanitizeValidTLSSecret(t *testing.T) {
	keyData := "-----BEGIN PRIVATE KEY-----\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCu9t8rBBx0VlnA\nrkIqmjUcdh+nqFje5w4hNCWAxwZYhmLioldyTVGTAoZ/ivP/q2sqKgoLIKu+9LYu\n1k762T5xuhWUfW+BD4mVQuoF6NxJlOW1UgttThXlDD0/MWjQ4CL4+iLhlr7OK3Pp\naQwlak0nlgO4U6eK2vTjkTbgZYXRnRusW7OSfYDlcUSqU7TrmwJuXvug6QobmPWY\nSdqaTG7cLqF+5Voy1w2JKhceQ1DGvUZ8qVFNuUZivytdEsGpnxaTEURK0scgroYX\npP/j1W9CV2eJYGI/YMGrtuMz0SRTP7zh/ZyUs3xtUcpnEkASuELd7GKWQ+JliLnH\nlTqBh/LfAgMBAAECggEAF7pWMKIUQ84UH6E3oEnH1c4f49+yUbnEZ59dTgDFF7X9\n7Mi3Eu1xGH6kA+GxznoOWf+tMK4jweKztFgPCk5HqCpkKSj2ZBUvLkVkBxzPMdFr\nckgn/Du16hlbUwRzN7ngiaMj84kQjlWZQ3h3aQ7osMDTof4CVOlLLcjbSC+sfW0s\nJacmcgP1rYVAcBo2IAo60WAxZBQzKbSHRSVS0g9khUoinfcdKDeyYGze//AiXXXR\nriJ9AYjGTsafV9+VCk6crcznk66eeJXAatecy4Jeu0k5vTta2GAeVkTviNQ4WlQA\nJIFd9uRAUAO5opra/MG9aDi62XcpvDTq8h6Clk5zqQKBgQD27KUIOrIlOnfeosGz\nIjNTMHXrDFG04t05jClSO+6D7mBr7l+t5rS7tPIsKtyibVcYggZKFa8jIYl30dxE\nL3/A8WVRRTZ6AckznzhyIbF1imTzK1sDbNSWwHvoICQ71DPO4nWwD/HnF5owgq9o\npAD/1vnAJLOgO9b/MsN4kip2twKBgQC1ZSRpjRkLgPO5uNxWVjSCf1s0fY9o6Byi\ngNmpsBwQ4by3+VOL+G0JrX1puZV/QL4vNZZ4fNo8xK9b16FmvWwImqadVr6AOzU7\nfMSHo5DUv+mXjA0KHBkdqbWLg3ppGicGYohnotq2lx+2Kubl039OdB+KtXF2OVgx\nnX2PKO19GQKBgGw1aF0i287UwJMgYCJQao2aPxKyY1wRz0DY24LeILhQTpD99ZAP\n+kQIF9ijL+0+XVywHnF47zdGCygnH5ACAMpc/zmOS0FMZw/oRqQ9f7cy3upxpYDq\nwH8P+zzOWRKe+9U+CLUPR8Mt5LQ9kQEaXhW/79L0QoOFtcJATMkZxOIhAoGBAIAe\ntQ48U5E1fnASKsZsUuBNNc0oVi+Bqh/5JEPfGKOv3UyQNLtrNxCb0jXnl7jusKXF\nksb9YGOFhFo5Pk3Dwtd86+u7hggqSZn/sQwgsj4iYsngaKFYYUD7SjgFIGO1zhSL\nac7RTuuiaAqR2M5BiOyPxmuBZmdbb3hzxWhlPwCZAoGAfYn3JgsVTp+DnXHOU9j5\nt1tKHLafi7nRVXmnHbFABmLC0KyJfWZqaCdUoJHRNhggAu918BWA2hlEWjmIG+id\nWiJ4y5oa63TrG/cr9zvdNHBy30KzvpK+aTgV5uPni7iV7URC1EEhghn6stB3LuvB\n0oPRUIeUnfrscWwxggtUGFE=\n-----END PRIVATE KEY-----\n"
	certData := "-----BEGIN CERTIFICATE-----\nMIIDLzCCAhegAwIBAgIUTDC2e9uCi0ggzWiI7XkkXNWmMY0wDQYJKoZIhvcNAQEL\nBQAwJzELMAkGA1UEBhMCVVMxGDAWBgNVBAMMD0V4YW1wbGUtZm9vLWJhcjAeFw0y\nMzEyMjgxMzUwNDNaFw0yNjEwMTcxMzUwNDNaMCcxCzAJBgNVBAYTAlVTMRgwFgYD\nVQQDDA9FeGFtcGxlLWZvby1iYXIwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEK\nAoIBAQCu9t8rBBx0VlnArkIqmjUcdh+nqFje5w4hNCWAxwZYhmLioldyTVGTAoZ/\nivP/q2sqKgoLIKu+9LYu1k762T5xuhWUfW+BD4mVQuoF6NxJlOW1UgttThXlDD0/\nMWjQ4CL4+iLhlr7OK3PpaQwlak0nlgO4U6eK2vTjkTbgZYXRnRusW7OSfYDlcUSq\nU7TrmwJuXvug6QobmPWYSdqaTG7cLqF+5Voy1w2JKhceQ1DGvUZ8qVFNuUZivytd\nEsGpnxaTEURK0scgroYXpP/j1W9CV2eJYGI/YMGrtuMz0SRTP7zh/ZyUs3xtUcpn\nEkASuELd7GKWQ+JliLnHlTqBh/LfAgMBAAGjUzBRMB0GA1UdDgQWBBTYsQEqc5CX\nzjBGv/O04Qd5sOu5QjAfBgNVHSMEGDAWgBTYsQEqc5CXzjBGv/O04Qd5sOu5QjAP\nBgNVHRMBAf8EBTADAQH/MA0GCSqGSIb3DQEBCwUAA4IBAQA1L7IsQ9GTFl9GQMGo\n+JOffZxhR9AiwnCPXTMWF8qYC99F39i946wJIkgJN6wm8Rt46gA65EfJw6YdjdB6\n8kjg3CDRDIFn2QRrP4x8tS4EBu9tUkss/2h0I16MEEB9RV8adjH0lkiPwQwP50uW\nwLlwMHw9KsxA1MATzSmBruzW//gyoJFaBKYsYqYa7VKcEyQqKgiQypBN2O01twF3\ntahLFTIeeD0e4fMe++mwJh8rT5sRpCLmFDIoajmLkjj48P7hvgtLFN+vRTqgqViq\nySngIMt75xyXeTm15o7LrEe4B4HkpWt4CbeUW/44HrCUoItuhyea7baGecLx8VoS\nR3xg\n-----END CERTIFICATE-----\n"

	fakeClient := fake.NewClientBuilder().Build()
	validator := New(fakeClient)

	cert := telemetryv1alpha1.ValueType{
		Value: certData,
	}

	key := telemetryv1alpha1.ValueType{
		Value: keyData,
	}

	err := validator.ValidateCertificate(context.Background(), &cert, &key)
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
		},
	}

	fakeClient := fake.NewClientBuilder().WithObjects(secret).Build()
	validator := New(fakeClient)

	tests := []struct {
		name        string
		inputCert   telemetryv1alpha1.ValueType
		inputKey    telemetryv1alpha1.ValueType
		expectedErr error
	}{
		{
			name: "cert missing",
			inputCert: telemetryv1alpha1.ValueType{ValueFrom: &telemetryv1alpha1.ValueFromSource{
				SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
					Name:      "unknown",
					Namespace: "default",
					Key:       "cert",
				}},
			},
			inputKey: telemetryv1alpha1.ValueType{ValueFrom: &telemetryv1alpha1.ValueFromSource{
				SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
					Name:      "test",
					Namespace: "default",
					Key:       "key",
				}},
			},
			expectedErr: ErrValueResolveFailed,
		},
		{
			name: "key missing",
			inputCert: telemetryv1alpha1.ValueType{ValueFrom: &telemetryv1alpha1.ValueFromSource{
				SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
					Name:      "test",
					Namespace: "default",
					Key:       "cert",
				}},
			},
			inputKey: telemetryv1alpha1.ValueType{ValueFrom: &telemetryv1alpha1.ValueFromSource{
				SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
					Name:      "unknown",
					Namespace: "default",
					Key:       "key",
				}},
			},
			expectedErr: ErrValueResolveFailed,
		},
		{
			name: "certs and key are present",
			inputCert: telemetryv1alpha1.ValueType{ValueFrom: &telemetryv1alpha1.ValueFromSource{
				SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
					Name:      "test",
					Namespace: "default",
					Key:       "cert",
				}},
			},
			inputKey: telemetryv1alpha1.ValueType{ValueFrom: &telemetryv1alpha1.ValueFromSource{
				SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
					Name:      "test",
					Namespace: "default",
					Key:       "key",
				}},
			},
			expectedErr: ErrCertDecodeFailed,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tt := test
			err := validator.ValidateCertificate(context.TODO(), &tt.inputCert, &tt.inputKey)
			require.ErrorIs(t, err, tt.expectedErr)
		})
	}

}

func TestInvalidCertificateKeyPair(t *testing.T) {
	_, clientCertsFoo, err := testutils.NewCertBuilder("foo", "fooNs").Build()
	require.NoError(t, err)
	_, clientCertsBar, err := testutils.NewCertBuilder("bar", "barNs").Build()
	require.NoError(t, err)

	keyData := clientCertsFoo.ClientKeyPem.String()
	certData := clientCertsBar.ClientCertPem.String()

	fakeClient := fake.NewClientBuilder().Build()
	validator := New(fakeClient)

	cert := telemetryv1alpha1.ValueType{
		Value: certData,
	}

	key := telemetryv1alpha1.ValueType{
		Value: keyData,
	}

	err = validator.ValidateCertificate(context.Background(), &cert, &key)
	require.ErrorIs(t, err, ErrInvalidCertificateKeyPair)

}
