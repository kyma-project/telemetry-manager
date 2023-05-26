package webhookcert

import (
	"context"
	"time"

	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

type certExpiryChecker interface {
	checkExpiry(ctx context.Context, certPEM []byte) (bool, error)
}

type certExpiryCheckerImpl struct {
	clock    clock
	timeLeft time.Duration
}

func (c *certExpiryCheckerImpl) checkExpiry(ctx context.Context, certPEM []byte) (bool, error) {
	cert, err := parseCertPEM(certPEM)
	if err != nil {
		return false, err
	}

	nowTime := c.clock.now()
	aboutToExpireTime := cert.NotAfter.UTC().Add(-1 * c.timeLeft)
	if nowTime.Before(aboutToExpireTime) {
		return true, nil
	}
	logf.FromContext(ctx).Error(err, "Cert is about to expire. Rotation is needed",
		"nowTime", nowTime,
		"aboutToExpireTime", aboutToExpireTime)
	return false, nil
}

type mockCertExpiryChecker struct {
	valid bool
	err   error
}

func (c *mockCertExpiryChecker) checkExpiry(context.Context, []byte) (bool, error) {
	return c.valid, c.err
}
