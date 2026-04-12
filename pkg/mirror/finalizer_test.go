package mirror

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/akii90/config-mirror/internal/constants"
)

func newFakeClient(objs ...runtime.Object) *fake.ClientBuilder {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	builder := fake.NewClientBuilder().WithScheme(scheme)
	for _, obj := range objs {
		builder = builder.WithRuntimeObjects(obj)
	}
	return builder
}

func TestEnsureFinalizer_Adds(t *testing.T) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "default",
		},
	}
	cl := newFakeClient(secret).Build()

	added, err := EnsureFinalizer(context.Background(), cl, secret)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !added {
		t.Error("expected added=true on first call")
	}
	// Finalizer should now be set on the in-memory object.
	found := false
	for _, f := range secret.GetFinalizers() {
		if f == constants.FinalizerCleanup {
			found = true
		}
	}
	if !found {
		t.Errorf("finalizer %q not found in object finalizers %v", constants.FinalizerCleanup, secret.GetFinalizers())
	}
}

func TestEnsureFinalizer_Idempotent(t *testing.T) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test",
			Namespace:  "default",
			Finalizers: []string{constants.FinalizerCleanup},
		},
	}
	cl := newFakeClient(secret).Build()

	added, err := EnsureFinalizer(context.Background(), cl, secret)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if added {
		t.Error("expected added=false when finalizer already present")
	}
}

func TestRemoveFinalizer_Removes(t *testing.T) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test",
			Namespace:  "default",
			Finalizers: []string{constants.FinalizerCleanup},
		},
	}
	cl := newFakeClient(secret).Build()

	if err := RemoveFinalizer(context.Background(), cl, secret); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, f := range secret.GetFinalizers() {
		if f == constants.FinalizerCleanup {
			t.Errorf("finalizer %q still present after removal", constants.FinalizerCleanup)
		}
	}
}

func TestRemoveFinalizer_NoOp(t *testing.T) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "default",
		},
	}
	cl := newFakeClient(secret).Build()

	// Should not error when finalizer is absent.
	if err := RemoveFinalizer(context.Background(), cl, secret); err != nil {
		t.Fatalf("expected no error when finalizer absent, got: %v", err)
	}
}

func TestRemoveFinalizer_PreservesOtherFinalizers(t *testing.T) {
	other := "some.other/finalizer"
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test",
			Namespace:  "default",
			Finalizers: []string{other, constants.FinalizerCleanup},
		},
	}
	cl := newFakeClient(secret).Build()

	if err := RemoveFinalizer(context.Background(), cl, secret); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	finalizers := secret.GetFinalizers()
	if len(finalizers) != 1 || finalizers[0] != other {
		t.Errorf("expected only %q to remain, got %v", other, finalizers)
	}
}
