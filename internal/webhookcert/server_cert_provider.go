package webhookcert

import (
	"context"
	"fmt"
	"time"
)

type serverCertGenerator interface {
	generateCert(host string, alternativeDNSNames []string, caCertPEM, caKeyPEM []byte) (certPEM, keyPEM []byte, err error)
}

type serverCertStorage interface {
	load() (certPEM, keyPEM []byte, err error)
	save(certPEM, keyPEM []byte) error
}

type serverCertProvider struct {
	checker   certExpiryChecker
	generator serverCertGenerator
	storage   serverCertStorage
}

func newServerCertProvider(certDir string) *serverCertProvider {
	clock := realClock{}
	const duration1d = 24 * time.Hour
	return &serverCertProvider{
		checker: &certExpiryCheckerImpl{timeLeft: duration1d, clock: realClock{}},
		generator: &serverCertGeneratorImpl{
			clock: clock,
		},
		storage: serverCertStorageImpl{certDir: certDir},
	}
}

func (p *serverCertProvider) provideCert(ctx context.Context, host string, alternativeDNSNames []string, caCertPEM, caKeyPEM []byte) ([]byte, []byte, error) {
	var err error
	var serverCertPEM, serverKeyPEM []byte
	serverCertPEM, serverKeyPEM, err = p.storage.load()
	shouldCreateNew := false
	if err != nil {
		shouldCreateNew = true
	} else {
		valid, checkErr := p.checker.checkExpiry(ctx, caCertPEM)
		if checkErr != nil {
			return nil, nil, fmt.Errorf("failed to check cert expiry: %w", checkErr)
		}
		shouldCreateNew = !valid
	}

	if shouldCreateNew {
		serverCertPEM, serverKeyPEM, err = p.generator.generateCert(host, alternativeDNSNames, caCertPEM, caKeyPEM)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to generate cert: %w", err)
		}

		if err = p.storage.save(serverCertPEM, serverKeyPEM); err != nil {
			return nil, nil, fmt.Errorf("failed to save server cert: %w", err)
		}
	}

	return serverCertPEM, serverKeyPEM, nil
}
