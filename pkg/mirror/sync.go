package mirror

import (
	"bytes"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/akii90/config-mirror/internal/constants"
)

// NeedsSyncSecret reports whether the target Secret's data differs from the source.
// Only .Data is compared; .StringData is write-only in the API and is always merged
// into .Data by the API server, so we only need to compare .Data.
func NeedsSyncSecret(source, target *corev1.Secret) bool {
	if len(source.Data) != len(target.Data) {
		return true
	}
	for k, v := range source.Data {
		if tv, ok := target.Data[k]; !ok || !bytes.Equal(v, tv) {
			return true
		}
	}
	return false
}

// NeedsSyncConfigMap reports whether the target ConfigMap's data differs from the source.
func NeedsSyncConfigMap(source, target *corev1.ConfigMap) bool {
	if len(source.Data) != len(target.Data) {
		return true
	}
	for k, v := range source.Data {
		if tv, ok := target.Data[k]; !ok || v != tv {
			return true
		}
	}
	if len(source.BinaryData) != len(target.BinaryData) {
		return true
	}
	for k, v := range source.BinaryData {
		if tv, ok := target.BinaryData[k]; !ok || !bytes.Equal(v, tv) {
			return true
		}
	}
	return false
}

// BuildMirrorSecret constructs a new Secret to be created in targetNamespace,
// copying data from source and injecting the required mirror labels/annotations.
func BuildMirrorSecret(source *corev1.Secret, targetNamespace string) *corev1.Secret {
	dataCopy := make(map[string][]byte, len(source.Data))
	for k, v := range source.Data {
		cp := make([]byte, len(v))
		copy(cp, v)
		dataCopy[k] = cp
	}

	return &corev1.Secret{
		ObjectMeta: mirrorObjectMeta(source.Namespace, source.Name, source.ResourceVersion, targetNamespace),
		Type:       source.Type,
		Data:       dataCopy,
	}
}

// BuildMirrorConfigMap constructs a new ConfigMap to be created in targetNamespace,
// copying data from source and injecting the required mirror labels/annotations.
func BuildMirrorConfigMap(source *corev1.ConfigMap, targetNamespace string) *corev1.ConfigMap {
	dataCopy := make(map[string]string, len(source.Data))
	for k, v := range source.Data {
		dataCopy[k] = v
	}
	binCopy := make(map[string][]byte, len(source.BinaryData))
	for k, v := range source.BinaryData {
		cp := make([]byte, len(v))
		copy(cp, v)
		binCopy[k] = cp
	}

	return &corev1.ConfigMap{
		ObjectMeta: mirrorObjectMeta(source.Namespace, source.Name, source.ResourceVersion, targetNamespace),
		Data:       dataCopy,
		BinaryData: binCopy,
	}
}

// mirrorObjectMeta builds the ObjectMeta for a mirrored resource.
func mirrorObjectMeta(sourceNamespace, sourceName, sourceResourceVersion, targetNamespace string) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:      sourceName,
		Namespace: targetNamespace,
		Labels: map[string]string{
			constants.LabelMirroredFrom: BuildMirroredFromValue(sourceNamespace, sourceName),
		},
		Annotations: map[string]string{
			constants.AnnotationSourceResourceVersion: sourceResourceVersion,
			constants.AnnotationMirroredAt:            time.Now().UTC().Format(time.RFC3339),
			// Store exact source coordinates so the drift-detection MapFunc can
			// enqueue the right reconcile request without string reversal.
			constants.AnnotationSourceNamespace: sourceNamespace,
			constants.AnnotationSourceName:      sourceName,
		},
	}
}

// ApplyMirrorSecret updates the fields of an existing target Secret to match the source.
// It preserves the target's ResourceVersion (required for updates) and updates
// the mirror annotations.
func ApplyMirrorSecret(source *corev1.Secret, target *corev1.Secret) {
	dataCopy := make(map[string][]byte, len(source.Data))
	for k, v := range source.Data {
		cp := make([]byte, len(v))
		copy(cp, v)
		dataCopy[k] = cp
	}
	target.Data = dataCopy
	target.Type = source.Type
	if target.Annotations == nil {
		target.Annotations = make(map[string]string)
	}
	target.Annotations[constants.AnnotationSourceResourceVersion] = source.ResourceVersion
	target.Annotations[constants.AnnotationMirroredAt] = time.Now().UTC().Format(time.RFC3339)
	target.Annotations[constants.AnnotationSourceNamespace] = source.Namespace
	target.Annotations[constants.AnnotationSourceName] = source.Name
	if target.Labels == nil {
		target.Labels = make(map[string]string)
	}
	target.Labels[constants.LabelMirroredFrom] = BuildMirroredFromValue(source.Namespace, source.Name)
}

// ApplyMirrorConfigMap updates the fields of an existing target ConfigMap to match the source.
func ApplyMirrorConfigMap(source *corev1.ConfigMap, target *corev1.ConfigMap) {
	dataCopy := make(map[string]string, len(source.Data))
	for k, v := range source.Data {
		dataCopy[k] = v
	}
	binCopy := make(map[string][]byte, len(source.BinaryData))
	for k, v := range source.BinaryData {
		cp := make([]byte, len(v))
		copy(cp, v)
		binCopy[k] = cp
	}
	target.Data = dataCopy
	target.BinaryData = binCopy
	if target.Annotations == nil {
		target.Annotations = make(map[string]string)
	}
	target.Annotations[constants.AnnotationSourceResourceVersion] = source.ResourceVersion
	target.Annotations[constants.AnnotationMirroredAt] = time.Now().UTC().Format(time.RFC3339)
	target.Annotations[constants.AnnotationSourceNamespace] = source.Namespace
	target.Annotations[constants.AnnotationSourceName] = source.Name
	if target.Labels == nil {
		target.Labels = make(map[string]string)
	}
	target.Labels[constants.LabelMirroredFrom] = BuildMirroredFromValue(source.Namespace, source.Name)
}
