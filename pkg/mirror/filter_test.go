package mirror

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func makeNS(name string, phase corev1.NamespacePhase) corev1.Namespace {
	return corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Status:     corev1.NamespaceStatus{Phase: phase},
	}
}

func nsNames(nsList []corev1.Namespace) []string {
	names := make([]string, len(nsList))
	for i, ns := range nsList {
		names[i] = ns.Name
	}
	return names
}

var baseNamespaces = []corev1.Namespace{
	makeNS("default", corev1.NamespaceActive),
	makeNS("app-prod", corev1.NamespaceActive),
	makeNS("app-staging", corev1.NamespaceActive),
	makeNS("kube-system", corev1.NamespaceActive),
	makeNS("istio-system", corev1.NamespaceActive),
	makeNS("source-ns", corev1.NamespaceActive),
	makeNS("dying", corev1.NamespaceTerminating),
}

func TestMatchNamespaces_NoPatterns(t *testing.T) {
	// No include/exclude → all active namespaces except source.
	got := MatchNamespaces(baseNamespaces, "source-ns", nil, nil)
	want := map[string]bool{
		"default": true, "app-prod": true, "app-staging": true,
		"kube-system": true, "istio-system": true,
	}
	if len(got) != len(want) {
		t.Errorf("got %v, want %d entries", got, len(want))
	}
	for _, name := range got {
		if !want[name] {
			t.Errorf("unexpected namespace %q in result", name)
		}
	}
}

func TestMatchNamespaces_SourceExcluded(t *testing.T) {
	got := MatchNamespaces(baseNamespaces, "source-ns", nil, nil)
	for _, name := range got {
		if name == "source-ns" {
			t.Error("source namespace should not appear in result")
		}
	}
}

func TestMatchNamespaces_TerminatingExcluded(t *testing.T) {
	got := MatchNamespaces(baseNamespaces, "source-ns", nil, nil)
	for _, name := range got {
		if name == "dying" {
			t.Error("terminating namespace should not appear in result")
		}
	}
}

func TestMatchNamespaces_IncludeOnly(t *testing.T) {
	got := MatchNamespaces(baseNamespaces, "source-ns", []string{"app-.*"}, nil)
	want := map[string]bool{"app-prod": true, "app-staging": true}
	if len(got) != len(want) {
		t.Errorf("got %v, want %v", got, nsNames(baseNamespaces))
	}
	for _, name := range got {
		if !want[name] {
			t.Errorf("unexpected namespace %q", name)
		}
	}
}

func TestMatchNamespaces_ExcludeOnly(t *testing.T) {
	got := MatchNamespaces(baseNamespaces, "source-ns", nil, []string{"kube-.*", "istio-.*"})
	for _, name := range got {
		if name == "kube-system" || name == "istio-system" {
			t.Errorf("excluded namespace %q appeared in result", name)
		}
	}
}

func TestMatchNamespaces_ExcludePriorityOverInclude(t *testing.T) {
	// include app-.* but exclude app-prod specifically.
	got := MatchNamespaces(baseNamespaces, "source-ns", []string{"app-.*"}, []string{"app-prod"})
	for _, name := range got {
		if name == "app-prod" {
			t.Error("app-prod should be excluded even though it matches include pattern")
		}
	}
	// app-staging should still be present.
	found := false
	for _, name := range got {
		if name == "app-staging" {
			found = true
		}
	}
	if !found {
		t.Error("app-staging should be in result")
	}
}

func TestMatchNamespaces_ExactMatch(t *testing.T) {
	got := MatchNamespaces(baseNamespaces, "source-ns", []string{"default"}, nil)
	if len(got) != 1 || got[0] != "default" {
		t.Errorf("got %v, want [default]", got)
	}
}

func TestMatchNamespaces_InvalidPatternSkipped(t *testing.T) {
	// An invalid regex should not panic; it is silently skipped (treated as no-match).
	got := MatchNamespaces(baseNamespaces, "source-ns", []string{"[invalid"}, nil)
	// Invalid include pattern → nothing matches → empty result.
	if len(got) != 0 {
		t.Errorf("expected empty result with invalid include pattern, got %v", got)
	}
}
