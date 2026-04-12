package mirror

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/akii90/config-mirror/internal/constants"
)

// --- NeedsSyncSecret ---

func TestNeedsSyncSecret_Identical(t *testing.T) {
	src := &corev1.Secret{Data: map[string][]byte{"key": []byte("val")}}
	tgt := &corev1.Secret{Data: map[string][]byte{"key": []byte("val")}}
	if NeedsSyncSecret(src, tgt) {
		t.Error("expected no sync needed for identical secrets")
	}
}

func TestNeedsSyncSecret_ValueDiffers(t *testing.T) {
	src := &corev1.Secret{Data: map[string][]byte{"key": []byte("new")}}
	tgt := &corev1.Secret{Data: map[string][]byte{"key": []byte("old")}}
	if !NeedsSyncSecret(src, tgt) {
		t.Error("expected sync needed when value differs")
	}
}

func TestNeedsSyncSecret_KeyAdded(t *testing.T) {
	src := &corev1.Secret{Data: map[string][]byte{"a": []byte("1"), "b": []byte("2")}}
	tgt := &corev1.Secret{Data: map[string][]byte{"a": []byte("1")}}
	if !NeedsSyncSecret(src, tgt) {
		t.Error("expected sync needed when source has extra key")
	}
}

func TestNeedsSyncSecret_BothEmpty(t *testing.T) {
	src := &corev1.Secret{}
	tgt := &corev1.Secret{}
	if NeedsSyncSecret(src, tgt) {
		t.Error("expected no sync needed for both empty secrets")
	}
}

// --- NeedsSyncConfigMap ---

func TestNeedsSyncConfigMap_Identical(t *testing.T) {
	src := &corev1.ConfigMap{Data: map[string]string{"k": "v"}}
	tgt := &corev1.ConfigMap{Data: map[string]string{"k": "v"}}
	if NeedsSyncConfigMap(src, tgt) {
		t.Error("expected no sync needed for identical configmaps")
	}
}

func TestNeedsSyncConfigMap_DataDiffers(t *testing.T) {
	src := &corev1.ConfigMap{Data: map[string]string{"k": "new"}}
	tgt := &corev1.ConfigMap{Data: map[string]string{"k": "old"}}
	if !NeedsSyncConfigMap(src, tgt) {
		t.Error("expected sync needed when data differs")
	}
}

func TestNeedsSyncConfigMap_BinaryDataDiffers(t *testing.T) {
	src := &corev1.ConfigMap{BinaryData: map[string][]byte{"b": {0x01}}}
	tgt := &corev1.ConfigMap{BinaryData: map[string][]byte{"b": {0x02}}}
	if !NeedsSyncConfigMap(src, tgt) {
		t.Error("expected sync needed when binary data differs")
	}
}

// --- BuildMirrorSecret ---

func TestBuildMirrorSecret_Labels(t *testing.T) {
	src := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       "source-ns",
			Name:            "my-secret",
			ResourceVersion: "42",
		},
		Data: map[string][]byte{"token": []byte("abc")},
	}
	got := BuildMirrorSecret(src, "target-ns")

	if got.Namespace != "target-ns" {
		t.Errorf("namespace = %q, want target-ns", got.Namespace)
	}
	if got.Name != "my-secret" {
		t.Errorf("name = %q, want my-secret", got.Name)
	}
	if got.Labels[constants.LabelMirroredFrom] == "" {
		t.Error("LabelMirroredFrom must be set")
	}
	if got.Annotations[constants.AnnotationSourceResourceVersion] != "42" {
		t.Errorf("AnnotationSourceResourceVersion = %q, want 42", got.Annotations[constants.AnnotationSourceResourceVersion])
	}
	if got.Annotations[constants.AnnotationMirroredAt] == "" {
		t.Error("AnnotationMirroredAt must be set")
	}
}

func TestBuildMirrorSecret_DataCopied(t *testing.T) {
	orig := []byte("secret-value")
	src := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "s"},
		Data:       map[string][]byte{"key": orig},
	}
	got := BuildMirrorSecret(src, "other")
	// Mutate original; mirror must not be affected.
	orig[0] = 'X'
	if got.Data["key"][0] == 'X' {
		t.Error("BuildMirrorSecret should deep-copy Data")
	}
}

// --- BuildMirrorConfigMap ---

func TestBuildMirrorConfigMap_LabelsAndData(t *testing.T) {
	src := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       "source-ns",
			Name:            "my-cm",
			ResourceVersion: "7",
		},
		Data:       map[string]string{"key": "val"},
		BinaryData: map[string][]byte{"bin": {0xDE, 0xAD}},
	}
	got := BuildMirrorConfigMap(src, "target-ns")

	if got.Namespace != "target-ns" {
		t.Errorf("namespace = %q, want target-ns", got.Namespace)
	}
	if got.Data["key"] != "val" {
		t.Errorf("Data[key] = %q, want val", got.Data["key"])
	}
	if len(got.BinaryData["bin"]) == 0 {
		t.Error("BinaryData must be copied")
	}
	if got.Labels[constants.LabelMirroredFrom] == "" {
		t.Error("LabelMirroredFrom must be set")
	}
}

// --- ApplyMirrorSecret ---

func TestApplyMirrorSecret_UpdatesData(t *testing.T) {
	src := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "s", ResourceVersion: "10"},
		Data:       map[string][]byte{"k": []byte("new")},
	}
	tgt := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: "other", Name: "s", ResourceVersion: "5"},
		Data:       map[string][]byte{"k": []byte("old")},
	}
	ApplyMirrorSecret(src, tgt)
	if string(tgt.Data["k"]) != "new" {
		t.Errorf("expected data updated to 'new', got %q", tgt.Data["k"])
	}
	// ResourceVersion of target must be preserved (needed for API update call).
	if tgt.ResourceVersion != "5" {
		t.Errorf("target ResourceVersion should not change, got %q", tgt.ResourceVersion)
	}
}
