package webhookcert

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCheckExpiry(t *testing.T) {
	ts := time.Now().UTC().Truncate(time.Hour)
	tests := []struct {
		summary          string
		certPEM          []byte
		now              time.Time
		softExpiryOffset time.Duration
		expectValid      bool
		expectError      bool
	}{
		{
			summary:     "nil input",
			expectError: true,
		},
		{
			summary:     "invalid input",
			certPEM:     []byte{1, 2, 3},
			expectError: true,
		},
		{
			summary:          "cert is soft-expired",
			certPEM:          generateCACert(ts), // validity interval [ts - 1 hour, ts + 365 days - 1 hour)
			now:              ts.Add(duration365d).Add(-2 * time.Hour),
			softExpiryOffset: 1 * time.Hour,
			expectValid:      false,
		},
		{
			summary:     "cert is hard-expired",
			certPEM:     generateCACert(ts), // validity interval [ts - 1 hour, ts + 365 days - 1 hour)
			now:         ts.Add(duration365d).Add(-1 * time.Hour),
			expectValid: false,
		},
		{
			summary:          "cert is not expired",
			certPEM:          generateCACert(ts), // validity interval [ts - 1 hour, ts + 365 days - 1 hour)
			now:              ts.Add(duration365d).Add(-2 * time.Hour).Add(-1 * time.Second),
			softExpiryOffset: 1 * time.Hour,
			expectValid:      true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.summary, func(t *testing.T) {
			sut := certExpiryCheckerImpl{
				clock:            mockClock{t: tc.now},
				softExpiryOffset: tc.softExpiryOffset,
			}
			valid, err := sut.checkExpiry(context.Background(), tc.certPEM)
			require.Equal(t, tc.expectValid, valid)
			if tc.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
