//go:build tools
// +build tools

package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
	"gopkg.in/yaml.v3"
)

type secScanConfig struct {
	ModuleName   string       `yaml:"module-name"`
	Kind         string       `yaml:"kind"`
	BDBA         []string     `yaml:"bdba"`
	Mend         mend         `yaml:"mend"`
	CheckmarxOne checkmarxOne `yaml:"checkmarx-one"`
}

type mend struct {
	Language string   `yaml:"language"`
	Exclude  []string `yaml:"exclude"`
}

type checkmarxOne struct {
	Preset  string   `yaml:"preset"`
	Exclude []string `yaml:"exclude"`
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	data, err := godotenv.Read(filepath.Join(".", ".env"))
	if err != nil {
		return fmt.Errorf("error reading .env file: %w", err)
	}

	if err := generateCommonConstants(data); err != nil {
		return fmt.Errorf("error generating common constants: %w", err)
	}

	if err := generateTestKitCommonConstants(data); err != nil {
		return fmt.Errorf("error generating testkit common constants: %w", err)
	}

	if err := generateSecScanConfig(data); err != nil {
		return fmt.Errorf("error generating sec scan config: %w", err)
	}

	return nil
}

func generateCommonConstants(data map[string]string) error {
	f, err := os.Create("./internal/images/images.go")
	if err != nil {
		return err
	}

	defer f.Close()
	_, err = f.WriteString(`// This file is generated by "make generate".
// Don't edit, update .env file and run make target generate.

package images

`)

	if err != nil {
		return err
	}

	_, err = f.WriteString("const (\n")
	if err != nil {
		return err
	}

	_, err = f.WriteString("\tDefaultFluentBitExporterImage = ")
	if err != nil {
		return err
	}

	_, err = f.WriteString(fmt.Sprintf("\"%s\"\n", data["DEFAULT_FLUENTBIT_EXPORTER_IMAGE"]))
	if err != nil {
		return err
	}

	_, err = f.WriteString("\tDefaultFluentBitImage         = ")
	if err != nil {
		return err
	}

	_, err = f.WriteString(fmt.Sprintf("\"%s\"\n", data["DEFAULT_FLUENTBIT_IMAGE"]))
	if err != nil {
		return err
	}

	_, err = f.WriteString("\tDefaultOTelCollectorImage     = ")
	if err != nil {
		return err
	}

	_, err = f.WriteString(fmt.Sprintf("\"%s\"\n", data["DEFAULT_OTEL_COLLECTOR_IMAGE"]))
	if err != nil {
		return err
	}

	_, err = f.WriteString("\tDefaultSelfMonitorImage       = ")
	if err != nil {
		return err
	}

	_, err = f.WriteString(fmt.Sprintf("\"%s\"\n", data["DEFAULT_SELFMONITOR_IMAGE"]))
	if err != nil {
		return err
	}

	_, err = f.WriteString(")\n")
	if err != nil {
		return err
	}

	return nil
}

func generateTestKitCommonConstants(data map[string]string) error {
	f, err := os.Create("./test/testkit/images.go")
	if err != nil {
		return err
	}

	defer f.Close()
	_, err = f.WriteString(`// This file is generated by "make generate".
// Don't edit, update .env file and run make target generate.

package testkit

`)

	if err != nil {
		return err
	}

	_, err = f.WriteString("const (\n")
	if err != nil {
		return err
	}

	_, err = f.WriteString("\tDefaultTelemetryGenImage = ")
	if err != nil {
		return err
	}

	_, err = f.WriteString(fmt.Sprintf("\"%s\"\n", data["DEFAULT_TEST_TELEMETRYGEN_IMAGE"]))
	if err != nil {
		return err
	}

	_, err = f.WriteString(")\n")
	if err != nil {
		return err
	}

	return nil
}

func generateSecScanConfig(data map[string]string) error {
	file, err := os.Create("./sec-scanners-config.yaml")
	if err != nil {
		return fmt.Errorf("error opening/creating file: %w", err)
	}
	defer file.Close()

	enc := yaml.NewEncoder(file)
	enc.SetIndent(2)

	imgs := []string{data["ENV_IMG"], data["DEFAULT_FLUENTBIT_EXPORTER_IMAGE"], data["DEFAULT_FLUENTBIT_IMAGE"], data["DEFAULT_OTEL_COLLECTOR_IMAGE"], data["DEFAULT_SELFMONITOR_IMAGE"]}
	secScanCfg := secScanConfig{
		ModuleName: "telemetry",
		Kind:       "kyma",
		BDBA:       imgs,
		Mend: mend{
			Language: "golang-mod",
			Exclude:  []string{"**/mocks/**", "**/stubs/**", "**/test/**", "**/*_test.go"},
		},
		CheckmarxOne: checkmarxOne{
			Preset:  "go-default",
			Exclude: []string{"**/mocks/**", "**/stubs/**", "**/test/**", "**/*_test.go"},
		},
	}

	err = enc.Encode(secScanCfg)

	if err != nil {
		return fmt.Errorf("error encoding: %w", err)
	}

	return nil
}
