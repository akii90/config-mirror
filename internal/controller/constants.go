package controller

// Re-export constants from the shared constants package so that controller
// code can reference them without a long import path.
import "github.com/akii90/config-mirror/internal/constants"

const (
	AnnotationAllowMirror           = constants.AnnotationAllowMirror
	AnnotationAllowMirrorEnabled    = constants.AnnotationAllowMirrorEnabled
	AnnotationNamespaceInclude      = constants.AnnotationNamespaceInclude
	AnnotationNamespaceExclude      = constants.AnnotationNamespaceExclude
	AnnotationSourceResourceVersion = constants.AnnotationSourceResourceVersion
	AnnotationMirroredAt            = constants.AnnotationMirroredAt
	AnnotationSourceNamespace       = constants.AnnotationSourceNamespace
	AnnotationSourceName            = constants.AnnotationSourceName
	LabelMirroredFrom               = constants.LabelMirroredFrom
	FinalizerCleanup                = constants.FinalizerCleanup
	LabelValueMaxLen                = constants.LabelValueMaxLen
	LabelValuePrefixLen             = constants.LabelValuePrefixLen
)
