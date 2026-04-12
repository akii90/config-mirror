package controller

const (
	// Annotations configured on Source resources to control mirror behavior.
	AnnotationAllowMirror       = "config-mirror.example.com/allow-mirror"
	AnnotationNamespaceInclude  = "config-mirror.example.com/namespace-include"
	AnnotationNamespaceExclude  = "config-mirror.example.com/namespace-exclude"

	// Annotations written onto mirrored (target) resources.
	AnnotationSourceResourceVersion = "config-mirror.example.com/source-resource-version"
	AnnotationMirroredAt            = "config-mirror.example.com/mirrored-at"

	// LabelMirroredFrom is the label key placed on mirrored resources to identify their source.
	// Value format: see pkg/mirror/label.go BuildMirroredFromValue.
	LabelMirroredFrom = "config-mirror.example.com/mirrored-from"

	// FinalizerCleanup is injected into Source resources that have mirroring enabled.
	// It ensures mirrored resources are deleted before the Source is removed from the cluster.
	FinalizerCleanup = "config-mirror.example.com/cleanup"

	// LabelValueMaxLen is the maximum length of a Kubernetes label value.
	LabelValueMaxLen = 63
	// LabelValuePrefixLen is the number of readable prefix characters kept when truncating.
	LabelValuePrefixLen = 52
)
