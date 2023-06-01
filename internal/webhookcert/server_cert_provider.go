package webhookcert

import (
	"context"
	"fmt"
	"time"

	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

type serverCertConfig struct {
	host                string
	alternativeDNSNames []string
	caCertPEM, caKeyPEM []byte
}

type serverCertGenerator interface {
	generateCert(config serverCertConfig) (certPEM, keyPEM []byte, err error)
}

type serverCertStorage interface {
	load() (certPEM, keyPEM []byte, err error)
	save(certPEM, keyPEM []byte) error
}

type serverCertProviderImpl struct {
	expiryChecker certExpiryChecker
	chainChecker  certChainChecker
	generator     serverCertGenerator
	storage       serverCertStorage
}

func newServerCertProvider(certDir string) *serverCertProviderImpl {
	clock := realClock{}
	const duration1d = 24 * time.Hour
	return &serverCertProviderImpl{
		expiryChecker: &certExpiryCheckerImpl{softExpiryOffset: duration1d, clock: realClock{}},
		chainChecker:  &certChainCheckerImpl{},
		generator: &serverCertGeneratorImpl{
			clock: clock,
		},
		storage: serverCertStorageImpl{certDir: certDir},
	}
}

func (p *serverCertProviderImpl) provideCert(ctx context.Context, config serverCertConfig) ([]byte, []byte, error) {
	var err error
	var serverCertPEM, serverKeyPEM []byte
	serverCertPEM, serverKeyPEM, err = p.storage.load()
	shouldCreateNew := false
	if err != nil || len(serverCertPEM) == 0 || len(serverKeyPEM) == 0 {
		shouldCreateNew = true
	} else {
		shouldCreateNew = !p.checkServerCert(ctx, serverCertPEM, config.caCertPEM)
	}

	if shouldCreateNew {
		logf.FromContext(ctx).Info("Generating new server cert/key")

		serverCertPEM, serverKeyPEM, err = p.generator.generateCert(config)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to generate cert: %w", err)
		}

		if err = p.storage.save(serverCertPEM, serverKeyPEM); err != nil {
			return nil, nil, fmt.Errorf("failed to save server cert: %w", err)
		}
	}

	return serverCertPEM, serverKeyPEM, nil
}

func (p *serverCertProviderImpl) checkServerCert(ctx context.Context, serverCertPEM, caCertPEM []byte) bool {
	var err error
	certValid := false

	certValid, err = p.expiryChecker.checkExpiry(ctx, serverCertPEM)
	if err != nil || !certValid {
		return false
	}

	certValid, err = p.chainChecker.checkRoot(ctx, serverCertPEM, caCertPEM)
	if err != nil || !certValid {
		return false
	}

	return true
}
