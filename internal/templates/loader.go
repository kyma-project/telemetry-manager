package templates

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type FileReader interface {
	ReadFile(filename string) ([]byte, error)
}

type OSFileReader struct{}

func (r *OSFileReader) ReadFile(filename string) ([]byte, error) {
	return os.ReadFile(filename)
}

type Loader struct {
	fileReader FileReader
}

func NewSpecTemplatesLoader(reader FileReader) *Loader {
	return &Loader{
		fileReader: reader,
	}
}
func (l *Loader) LoadPodSpecTemplate(fileName string) (*v1.PodTemplateSpec, error) {
	podTemplate, err := l.fileReader.ReadFile(fileName)
	if err != nil {
		return nil, fmt.Errorf("failed to read pod template file: %w", err)
	}

	var tpl v1.PodTemplateSpec
	err = yaml.Unmarshal(podTemplate, &tpl)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal pod template: %w", err)
	}
	return &tpl, nil
}

func (l *Loader) LoadMetadataTemplate(fileName string) (*metav1.ObjectMeta, error) {
	metadataTemplate, err := l.fileReader.ReadFile(fileName)
	if err != nil {
		return nil, fmt.Errorf("failed to read metadata template file: %w", err)
	}

	var tpl metav1.ObjectMeta
	err = yaml.Unmarshal(metadataTemplate, &tpl)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal metadata template: %w", err)
	}
	return &tpl, nil
}
