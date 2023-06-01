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
	clock            clock
	softExpiryOffset time.Duration
}

// checkExpiry checks if the provided PEM-encoded certificate is expired
// if softExpiryOffset is set, it checks if it's soft-expired (before the actual hard expiration date)
func (c *certExpiryCheckerImpl) checkExpiry(ctx context.Context, certPEM []byte) (bool, error) {
	cert, err := parseCertPEM(certPEM)
	if err != nil {
		return false, err
	}

	nowTime := c.clock.now().UTC()
	hardExpiryTime := cert.NotAfter.UTC()
	softExpiryTime := hardExpiryTime.Add(-1 * c.softExpiryOffset)
	if nowTime.Before(softExpiryTime) {
		return true, nil
	}
	logf.FromContext(ctx).Info("Cert expiry check failed",
		"nowTime", nowTime,
		"hardExpiryTime", hardExpiryTime,
		"softExpiryTime", softExpiryTime)
	return false, nil
}
