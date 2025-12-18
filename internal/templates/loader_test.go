package templates

import (
	"errors"
	"testing"
)

type mockReader struct {
	data []byte
	err  error
}

func (m *mockReader) ReadFile(filename string) ([]byte, error) {
	if m.err != nil {
		return nil, m.err
	}

	return m.data, nil
}

func TestLoadPodSpecTemplate(t *testing.T) {
	cases := []struct {
		name      string
		reader    FileReader
		wantImage string
		wantErr   bool
	}{
		{
			name:   "success",
			reader: &mockReader{data: []byte("objectmeta:\n  labels:\n    foo: bar\n  annotations:\n    anno-foo: bar\nspec:\n  containers:\n  - name: c1\n    image: nginx\n")},

			wantImage: "nginx",
			wantErr:   false,
		},
		{
			name:    "read error",
			reader:  &mockReader{err: errors.New("read failure")},
			wantErr: true,
		},
		{
			name:    "unmarshal error",
			reader:  &mockReader{data: []byte("::::invalid yaml")},
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			loader := NewSpecTemplatesLoader(tc.reader)

			tpl, err := loader.LoadPodSpecTemplate("file.yaml")
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}

				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tpl.ObjectMeta.Labels["foo"] != "bar" {
				t.Fatalf("label mismatch: want %q got %q", "bar", tpl.ObjectMeta.Labels["foo"])
			}

			if tpl.ObjectMeta.Annotations["anno-foo"] != "bar" {
				t.Fatalf("annotation mismatch: want %q got %q", "bar", tpl.ObjectMeta.Annotations["anno-foo"])
			}

			if len(tpl.Spec.Containers) == 0 {
				t.Fatalf("expected at least one container")
			}

			if tpl.Spec.Containers[0].Image != tc.wantImage {
				t.Fatalf("image mismatch: want %q got %q", tc.wantImage, tpl.Spec.Containers[0].Image)
			}
		})
	}
}

func TestLoadMetadataTemplate(t *testing.T) {
	cases := []struct {
		name      string
		reader    FileReader
		wantName  string
		wantLabel string
		wantErr   bool
	}{
		{
			name:      "success",
			reader:    &mockReader{data: []byte("name: mymeta\nlabels:\n  test/label: label-value\n")},
			wantName:  "mymeta",
			wantLabel: "label-value",
			wantErr:   false,
		},
		{
			name:    "read error",
			reader:  &mockReader{err: errors.New("read failure")},
			wantErr: true,
		},
		{
			name:    "unmarshal error",
			reader:  &mockReader{data: []byte("::bad yaml")},
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			loader := NewSpecTemplatesLoader(tc.reader)

			meta, err := loader.LoadMetadataTemplate("file.yaml")
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}

				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if meta.Name != tc.wantName {
				t.Fatalf("name mismatch: want %q got %q", tc.wantName, meta.Name)
			}

			if got := meta.Labels["test/label"]; got != tc.wantLabel {
				t.Fatalf("label mismatch: want %q got %q", tc.wantLabel, got)
			}
		})
	}
}
