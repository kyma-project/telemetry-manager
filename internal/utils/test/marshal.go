package test

import (
	"bytes"
	"fmt"
	"slices"
	"strings"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	apiutils "sigs.k8s.io/controller-runtime/pkg/client/apiutil"
)

func MarshalYAML(objects []client.Object) ([]byte, error) {
	scheme := runtime.NewScheme()
	clientgoscheme.AddToScheme(scheme)

	//TypeMeta is not set by default, so we need to set it manually
	for _, obj := range objects {
		gvk, err := apiutils.GVKForObject(obj, scheme)
		if err != nil {
			return nil, fmt.Errorf("failed to get GVK for object %T: %w", obj, err)
		}

		obj.GetObjectKind().SetGroupVersionKind(gvk)
	}

	//Always sort to have a deterministic output
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
