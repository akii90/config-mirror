package mirror

import (
	"regexp"

	corev1 "k8s.io/api/core/v1"
)

// MatchNamespaces returns the subset of namespaces that should receive a mirrored
// resource, applying the following rules in order:
//
//  1. Exclude the source namespace itself.
//  2. Exclude namespaces in Terminating phase.
//  3. If includePatterns is non-empty, keep only namespaces matching at least one
//     successfully compiled pattern. If all patterns are invalid, nothing is included.
//  4. Exclude namespaces matching any excludePattern (higher priority than include).
//
// Each pattern is treated as a full-match regular expression (anchored with ^ and $).
// An empty includePatterns list means "all namespaces".
func MatchNamespaces(nsList []corev1.Namespace, sourceNamespace string, includePatterns, excludePatterns []string) []string {
	includeREs := compilePatterns(includePatterns)
	excludeREs := compilePatterns(excludePatterns)

	// hasIncludeFilter is true when the caller specified at least one include pattern,
	// regardless of whether any compiled successfully. This prevents invalid patterns
	// from silently becoming "include all".
	hasIncludeFilter := len(includePatterns) > 0

	var result []string
	for _, ns := range nsList {
		name := ns.Name

		// Rule 1: skip source namespace.
		if name == sourceNamespace {
			continue
		}

		// Rule 2: skip terminating namespaces.
		if ns.Status.Phase == corev1.NamespaceTerminating {
			continue
		}

		// Rule 3: must match at least one include pattern (if any specified).
		if hasIncludeFilter && !matchesAny(name, includeREs) {
			continue
		}

		// Rule 4: must not match any exclude pattern.
		if matchesAny(name, excludeREs) {
			continue
		}

		result = append(result, name)
	}
	return result
}

// compilePatterns compiles each pattern as a full-match regex (^pattern$).
// Invalid patterns are silently skipped.
func compilePatterns(patterns []string) []*regexp.Regexp {
	res := make([]*regexp.Regexp, 0, len(patterns))
	for _, p := range patterns {
		re, err := regexp.Compile("^(?:" + p + ")$")
		if err != nil {
			continue
		}
		res = append(res, re)
	}
	return res
}

// matchesAny returns true if name matches any of the compiled regular expressions.
func matchesAny(name string, res []*regexp.Regexp) bool {
	for _, re := range res {
		if re.MatchString(name) {
			return true
		}
	}
	return false
}
