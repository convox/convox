package k8s

import (
	"testing"

	apps "k8s.io/api/apps/v1"
	ac "k8s.io/api/core/v1"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestIsPdbDisabled(t *testing.T) {
	tests := []struct {
		name           string
		deployAnnots   map[string]string
		templateAnnots map[string]string
		want           bool
	}{
		{
			name: "no annotations",
			want: false,
		},
		{
			name:         "new spelling on deployment",
			deployAnnots: map[string]string{AnnotationPdbDisabled: "true"},
			want:         true,
		},
		{
			name:           "new spelling on template",
			templateAnnots: map[string]string{AnnotationPdbDisabled: "true"},
			want:           true,
		},
		{
			name:         "old spelling on deployment",
			deployAnnots: map[string]string{AnnotationPdbDisabledDeprecated: "true"},
			want:         true,
		},
		{
			name:           "old spelling on template",
			templateAnnots: map[string]string{AnnotationPdbDisabledDeprecated: "true"},
			want:           true,
		},
		{
			name:           "both spellings both true",
			deployAnnots:   map[string]string{AnnotationPdbDisabled: "true"},
			templateAnnots: map[string]string{AnnotationPdbDisabledDeprecated: "true"},
			want:           true,
		},
		{
			name:         "value is false",
			deployAnnots: map[string]string{AnnotationPdbDisabled: "false"},
			want:         false,
		},
		{
			name:         "value is empty string",
			deployAnnots: map[string]string{AnnotationPdbDisabled: ""},
			want:         false,
		},
		{
			name:         "value is 1",
			deployAnnots: map[string]string{AnnotationPdbDisabled: "1"},
			want:         false,
		},
		{
			name:         "unrelated annotations only",
			deployAnnots: map[string]string{"foo": "bar"},
			want:         false,
		},
		{
			name:           "mixed conflict new false deploy old true template",
			deployAnnots:   map[string]string{AnnotationPdbDisabled: "false"},
			templateAnnots: map[string]string{AnnotationPdbDisabledDeprecated: "true"},
			want:           true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &apps.Deployment{
				ObjectMeta: am.ObjectMeta{Annotations: tt.deployAnnots},
				Spec: apps.DeploymentSpec{
					Template: ac.PodTemplateSpec{
						ObjectMeta: am.ObjectMeta{Annotations: tt.templateAnnots},
					},
				},
			}
			if got := isPdbDisabled(d); got != tt.want {
				t.Errorf("isPdbDisabled() = %v, want %v", got, tt.want)
			}
		})
	}
}
