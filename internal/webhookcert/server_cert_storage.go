package webhookcert

import (
	"fmt"
	"os"
	"path"
)

const (
	certFile = "tls.crt"
	keyFile  = "tls.key"
)

type serverCertStorageImpl struct {
	certDir string
}

func (s serverCertStorageImpl) load() ([]byte, []byte, error) {
	var err error
	var certPEM, keyPEM []byte
	certPEM, err = os.ReadFile(s.certFilePath())
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load cert: %w", err)
	}
	keyPEM, err = os.ReadFile(s.keyFilePath())
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load key: %w", err)
	}
	return certPEM, keyPEM, nil
}

func (s serverCertStorageImpl) save(certPEM, keyPEM []byte) error {
	if err := os.WriteFile(s.certFilePath(), certPEM, 0600); err != nil {
		return fmt.Errorf("failed to save cert: %w", err)
	}
	if err := os.WriteFile(s.keyFilePath(), keyPEM, 0600); err != nil {
		return fmt.Errorf("failed to save key: %w", err)
	}
	return nil
}

func (s serverCertStorageImpl) certFilePath() string {
	return path.Join(s.certDir, certFile)
}

func (s serverCertStorageImpl) keyFilePath() string {
	return path.Join(s.certDir, keyFile)
}
