// Package constants defines shared string constants used by both the controller
// and the mirror utility packages, avoiding import cycles.
package constants

const (
	// Annotations configured on Source resources to control mirror behavior.
	AnnotationAllowMirror = "config-mirror.example.com/allow-mirror"
	// AnnotationAllowMirrorEnabled is the value that activates mirroring.
	AnnotationAllowMirrorEnabled = "true"
	AnnotationNamespaceInclude   = "config-mirror.example.com/namespace-include"
	AnnotationNamespaceExclude   = "config-mirror.example.com/namespace-exclude"

	// Annotations written onto mirrored (target) resources.
	AnnotationSourceResourceVersion = "config-mirror.example.com/source-resource-version"
	AnnotationMirroredAt            = "config-mirror.example.com/mirrored-at"
	// AnnotationSourceNamespace and AnnotationSourceName record the exact origin of a
	// mirrored resource so the drift-detection MapFunc can enqueue the right reconcile
	// request without string reversal.
	AnnotationSourceNamespace = "config-mirror.example.com/source-namespace"
	AnnotationSourceName      = "config-mirror.example.com/source-name"

	// LabelMirroredFrom is placed on mirrored resources to identify their source.
	// Value format: see pkg/mirror BuildMirroredFromValue.
	LabelMirroredFrom = "config-mirror.example.com/mirrored-from"

	// FinalizerCleanup is injected into Source resources with mirroring enabled.
	FinalizerCleanup = "config-mirror.example.com/cleanup"

	// LabelValueMaxLen is the maximum length of a Kubernetes label value.
	LabelValueMaxLen = 63
	// LabelValuePrefixLen is the number of readable prefix characters kept when truncating.
	LabelValuePrefixLen = 52
)
