package test

import (
	"bytes"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
)

// MarshalYAML marshals a list of objects into a YAML byte slice. It is used to compare the expected objects with the actual ones in golden file tests.
func MarshalYAML(scheme *runtime.Scheme, objects []client.Object) ([]byte, error) {
	// TypeMeta is not set by default, so we need to set it manually
	for _, obj := range objects {
		gvk, err := apiutil.GVKForObject(obj, scheme)
		if err != nil {
			return nil, fmt.Errorf("failed to get GVK for object %T: %w", obj, err)
		}

		obj.GetObjectKind().SetGroupVersionKind(gvk)
	}

	// Always sort to have a deterministic output
	slices.SortFunc(objects, compareObjects)

	e := json.NewYAMLSerializer(json.DefaultMetaFactory, scheme, scheme)

	var buffer bytes.Buffer

	for _, obj := range objects {
		if err := e.Encode(obj, &buffer); err != nil {
			return nil, fmt.Errorf("failed to encode object %T: %w", obj, err)
		}

		buffer.WriteString("---\n") // YAML document separator
	}

	return buffer.Bytes(), nil
}

// SaveAsYAML dumps the list of objects as a YAML file. Used to generate golden files, never should be committed.
func SaveAsYAML(t *testing.T, scheme *runtime.Scheme, objects []client.Object, path string) {
	objectsYAML, err := MarshalYAML(scheme, objects)
	require.NoError(t, err)

	err = os.WriteFile(path, objectsYAML, 0600)
	require.NoError(t, err)

	t.Fatalf("Golden file %s has been saved, please verify it and remove this line", path)
}

func compareObjects(a, b client.Object) int {
	gvkA := a.GetObjectKind().GroupVersionKind()
	gvkB := b.GetObjectKind().GroupVersionKind()

	if cmp := compareGVKs(gvkA, gvkB); cmp != 0 {
		return cmp
	}

	if cmp := strings.Compare(a.GetNamespace(), b.GetNamespace()); cmp != 0 {
		return cmp
	}

	return strings.Compare(a.GetName(), b.GetName())
}

func compareGVKs(a, b schema.GroupVersionKind) int {
	if cmp := strings.Compare(a.Group, b.Group); cmp != 0 {
		return cmp
	}

	if cmp := strings.Compare(a.Version, b.Version); cmp != 0 {
		return cmp
	}

	return strings.Compare(a.Kind, b.Kind)
}
