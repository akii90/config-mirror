package mirror

import (
	"context"
	"slices"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/akii90/config-mirror/internal/controller"
)

// EnsureFinalizer adds the cleanup finalizer to obj if it is not already present.
// It returns (true, nil) when the finalizer was added and the object was updated,
// (false, nil) when the finalizer was already present (no API call made),
// or (false, err) on an update failure.
func EnsureFinalizer(ctx context.Context, c client.Client, obj client.Object) (added bool, err error) {
	if slices.Contains(obj.GetFinalizers(), controller.FinalizerCleanup) {
		return false, nil
	}
	obj.SetFinalizers(append(obj.GetFinalizers(), controller.FinalizerCleanup))
	if err := c.Update(ctx, obj); err != nil {
		return false, err
	}
	return true, nil
}

// RemoveFinalizer removes the cleanup finalizer from obj and updates it via the API.
// If the finalizer is not present, this is a no-op and returns nil.
func RemoveFinalizer(ctx context.Context, c client.Client, obj client.Object) error {
	finalizers := obj.GetFinalizers()
	idx := slices.Index(finalizers, controller.FinalizerCleanup)
	if idx == -1 {
		return nil
	}
	obj.SetFinalizers(slices.Delete(finalizers, idx, idx+1))
	return c.Update(ctx, obj)
}
