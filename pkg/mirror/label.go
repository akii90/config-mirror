package mirror

import (
	"crypto/sha256"
	"fmt"
	"regexp"
	"strings"

	"github.com/akii90/config-mirror/internal/controller"
)

// invalidLabelChars matches any character that is not allowed in a Kubernetes label value.
// Allowed: [a-z0-9A-Z] and the separators [-_.] (but not at the start or end).
var invalidLabelChars = regexp.MustCompile(`[^a-zA-Z0-9\-_.]`)

// BuildMirroredFromValue returns the value for the LabelMirroredFrom label.
//
// The value is derived from "<sourceNamespace>-<sourceName>" after sanitizing
// invalid characters. If the result exceeds 63 characters (Kubernetes limit),
// a truncated form is used:
//
//	<first 52 chars>-<SHA256 hex prefix of 10 chars>
func BuildMirroredFromValue(sourceNamespace, sourceName string) string {
	raw := sourceNamespace + "-" + sourceName
	sanitized := sanitizeLabelValue(raw)

	if len(sanitized) <= controller.LabelValueMaxLen {
		return sanitized
	}

	// Truncate: keep first LabelValuePrefixLen chars + "-" + 10-char SHA256 prefix.
	hash := sha256.Sum256([]byte(raw))
	hashHex := fmt.Sprintf("%x", hash[:5]) // 5 bytes → 10 hex chars

	prefix := sanitized[:controller.LabelValuePrefixLen]
	// Trim any trailing separator characters from the prefix to avoid invalid label endings.
	prefix = strings.TrimRight(prefix, "-_.")

	return prefix + "-" + hashHex
}

// sanitizeLabelValue replaces characters that are not allowed in a Kubernetes label value
// with a hyphen, then trims any leading/trailing separators.
func sanitizeLabelValue(s string) string {
	s = strings.ToLower(s)
	s = invalidLabelChars.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-_.")
	return s
}
