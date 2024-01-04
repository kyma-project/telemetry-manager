package tlsvalidation

import (
	"crypto/tls"
	"strings"
)

func SanitizeTLSSecret(tlsCert, tlsKey []byte) ([]byte, []byte) {
	_, err := tls.X509KeyPair(tlsCert, tlsKey)
	if err != nil {
		certReplaced := []byte(strings.ReplaceAll(string(tlsCert), "\\n", "\n"))
		keyReplaced := []byte(strings.ReplaceAll(string(tlsKey), "\\n", "\n"))
		return certReplaced, keyReplaced

	}

	return tlsCert, tlsKey
}
