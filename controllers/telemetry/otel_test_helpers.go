package telemetry

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

func validatePodMetadata(deployment appsv1.Deployment) error {
	if value, found := deployment.Spec.Template.ObjectMeta.Labels["sidecar.istio.io/inject"]; !found || value != "false" {
		return fmt.Errorf("istio sidecar injection for otel collector not disabled")
	}

	if value, found := deployment.Spec.Template.ObjectMeta.Annotations["checksum/config"]; !found || value == "" {
		return fmt.Errorf("configuration hash not found in pod annotations")
	}

	return nil
}

func validateSecret(secret corev1.Secret, expectedUsername, expectedPassword string) error {
	authHeader := secret.Data["BASIC_AUTH_HEADER_DUMMY"]
	if authHeader == nil {
		return fmt.Errorf("the key 'Authorization' is not in secret '%s'", secret.Name)
	}

	username, password, err := getAuthInfoFromHeader(authHeader)
	if err != nil {
		return err
	}

	if username != expectedUsername {
		return fmt.Errorf("extracted username is not equal to expected: %s != %s", username, expectedPassword)
	}

	if password != expectedPassword {
		return fmt.Errorf("extracted username is not equal to expected: %s != %s", username, expectedPassword)
	}
	return nil
}

func validateSecretDataWithKey(secret corev1.Secret, secretKey, expected string) error {
	secretData := secret.Data[secretKey]
	if secretData == nil {
		return fmt.Errorf("the key '%s' is not in secret '%s'", secretKey, secret.Name)
	}

	if string(secretData) != expected {
		return fmt.Errorf("extracted data is not equal to expected: %s != %s", string(secretData), expected)
	}
	return nil
}

func getAuthInfoFromHeader(header []byte) (string, string, error) {
	trimmedHeader := strings.TrimPrefix(string(header), "Basic ")
	decodedHeader, err := base64.StdEncoding.DecodeString(trimmedHeader)
	if err != nil {
		return "", "", fmt.Errorf("could not decode Authorization Header: %w", err)
	}

	splitHeader := strings.Split(string(decodedHeader), ":")
	if len(splitHeader) != 2 {
		return "", "", errors.New("decoded Authorization Header is invalid")
	}
	username := splitHeader[0]
	password := splitHeader[1]
	return username, password, nil
}
