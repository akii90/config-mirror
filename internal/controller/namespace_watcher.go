package controller

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// NamespaceReconciler watches Namespace creation events and fans out Reconcile
// requests to all Source resources (Secrets and ConfigMaps) that have mirroring
// enabled. This ensures that a newly created Namespace receives its copies of
// all relevant mirrored resources without waiting for the source to be updated.
type NamespaceReconciler struct {
	client.Client
	// MirrorQueue is used to enqueue Reconcile requests into the MirrorReconciler's
	// workqueue. It is populated in SetupWithManager via the MirrorReconciler controller.
}

// Reconcile is called when a Namespace event is observed.
// It lists all Secrets and ConfigMaps that have allow-mirror=true and enqueues
// them for reconciliation through the mirror controller by returning the requests.
//
// Note: the fan-out is implemented via EnqueueRequestsFromMapFunc in SetupWithManager,
// so this Reconcile method itself is a no-op—the real work happens in MirrorReconciler.
func (r *NamespaceReconciler) Reconcile(_ context.Context, _ ctrl.Request) (ctrl.Result, error) {
	// The actual reconciliation is performed by MirrorReconciler.
	// This reconciler exists solely to trigger fan-out via the MapFunc registered
	// in MirrorReconciler.SetupWithManager.
	return ctrl.Result{}, nil
}

// namespaceBecameActivePredicate allows only events for Active namespaces.
// Create events for Active namespaces trigger fan-out; non-Active and Delete events
// are ignored.
var namespaceBecameActivePredicate = predicate.Funcs{
	CreateFunc: func(e event.CreateEvent) bool {
		ns, ok := e.Object.(*corev1.Namespace)
		return ok && ns.Status.Phase == corev1.NamespaceActive
	},
	UpdateFunc: func(e event.UpdateEvent) bool {
		oldNS, ok1 := e.ObjectOld.(*corev1.Namespace)
		newNS, ok2 := e.ObjectNew.(*corev1.Namespace)
		if !ok1 || !ok2 {
			return false
		}
		// Only trigger when a namespace transitions into Active.
		return oldNS.Status.Phase != corev1.NamespaceActive &&
			newNS.Status.Phase == corev1.NamespaceActive
	},
	DeleteFunc:  func(_ event.DeleteEvent) bool { return false },
	GenericFunc: func(_ event.GenericEvent) bool { return false },
}

// NewNamespaceToSourcesMapFunc returns a MapFunc that, for a given Namespace event,
// lists all Secrets and ConfigMaps with allow-mirror=true from the cache and returns
// Reconcile requests for each of them (targeting the MirrorReconciler).
func NewNamespaceToSourcesMapFunc(c client.Client) handler.MapFunc {
	return func(ctx context.Context, _ client.Object) []reconcile.Request {
		log := ctrl.LoggerFrom(ctx)
		var requests []reconcile.Request

		// Fan out to all mirroring-enabled Secrets.
		secretList := &corev1.SecretList{}
		if err := c.List(ctx, secretList, client.MatchingFields{
			allowMirrorIndexField: "true",
		}); err != nil {
			log.Error(err, "failed to list mirroring secrets for namespace fan-out")
		} else {
			for _, s := range secretList.Items {
				requests = append(requests, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Namespace: s.Namespace,
						Name:      s.Name,
					},
				})
			}
		}

		// Fan out to all mirroring-enabled ConfigMaps.
		cmList := &corev1.ConfigMapList{}
		if err := c.List(ctx, cmList, client.MatchingFields{
			allowMirrorIndexField: "true",
		}); err != nil {
			log.Error(err, "failed to list mirroring configmaps for namespace fan-out")
		} else {
			for _, cm := range cmList.Items {
				requests = append(requests, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Namespace: cm.Namespace,
						Name:      cm.Name,
					},
				})
			}
		}

		return requests
	}
}

// allowMirrorIndexField is the field index name used to efficiently query
// resources with allow-mirror=true from the informer cache.
const allowMirrorIndexField = ".metadata.annotations.allow-mirror"

// SetupWithManager registers the Namespace watch and the field indexers needed
// for the namespace-to-sources fan-out. The fan-out requests are sent to the
// MirrorReconciler's queue by registering the Namespace watch on it directly
// via RegisterNamespaceWatch.
func (r *NamespaceReconciler) SetupIndexers(mgr ctrl.Manager) error {
	// Index Secret by allow-mirror annotation value.
	if err := mgr.GetFieldIndexer().IndexField(
		context.Background(),
		&corev1.Secret{},
		allowMirrorIndexField,
		func(obj client.Object) []string {
			val := obj.GetAnnotations()[AnnotationAllowMirror]
			if val == "" {
				return nil
			}
			return []string{val}
		},
	); err != nil {
		return err
	}

	// Index ConfigMap by allow-mirror annotation value.
	return mgr.GetFieldIndexer().IndexField(
		context.Background(),
		&corev1.ConfigMap{},
		allowMirrorIndexField,
		func(obj client.Object) []string {
			val := obj.GetAnnotations()[AnnotationAllowMirror]
			if val == "" {
				return nil
			}
			return []string{val}
		},
	)
}

// SetupWithManager registers a minimal controller for the NamespaceReconciler itself.
// The actual fan-out watch on Namespace is added to the MirrorReconciler via
// RegisterNamespaceWatch so that Namespace events enqueue into the mirror workqueue.
func (r *NamespaceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		Named("namespace-watcher").
		For(&corev1.Namespace{}, builder.WithPredicates(namespaceBecameActivePredicate)).
		Complete(r)
}
