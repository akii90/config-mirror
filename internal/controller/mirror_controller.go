package controller

import (
	"context"
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/akii90/config-mirror/pkg/mirror"
)

// MirrorReconciler reconciles Secret and ConfigMap resources that carry
// the allow-mirror annotation, syncing their contents to target namespaces.
type MirrorReconciler struct {
	client.Client
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=namespaces,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=events,verbs=create;patch

func (r *MirrorReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	// --- Determine resource type: try Secret first, then ConfigMap. ---

	secret := &corev1.Secret{}
	if err := r.Get(ctx, req.NamespacedName, secret); err == nil {
		return r.reconcileSecret(ctx, secret)
	} else if !apierrors.IsNotFound(err) {
		return ctrl.Result{}, fmt.Errorf("get secret %s: %w", req.NamespacedName, err)
	}

	cm := &corev1.ConfigMap{}
	if err := r.Get(ctx, req.NamespacedName, cm); err == nil {
		return r.reconcileConfigMap(ctx, cm)
	} else if !apierrors.IsNotFound(err) {
		return ctrl.Result{}, fmt.Errorf("get configmap %s: %w", req.NamespacedName, err)
	}

	log.Info("resource not found, likely deleted", "namespacedName", req.NamespacedName)
	return ctrl.Result{}, nil
}

// ---- Secret reconciliation -----------------------------------------------

func (r *MirrorReconciler) reconcileSecret(ctx context.Context, src *corev1.Secret) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx).WithValues("source", src.Namespace+"/"+src.Name)

	mirrorEnabled := src.Annotations[AnnotationAllowMirror] == "true"

	// Handle deletion (DeletionTimestamp set) or mirror disabled.
	if !src.DeletionTimestamp.IsZero() || !mirrorEnabled {
		if err := r.cleanupMirroredSecrets(ctx, src.Namespace, src.Name); err != nil {
			return ctrl.Result{}, err
		}
		if err := mirror.RemoveFinalizer(ctx, r.Client, src); err != nil {
			return ctrl.Result{}, fmt.Errorf("remove finalizer from secret: %w", err)
		}
		if mirrorEnabled {
			r.Recorder.Eventf(src, corev1.EventTypeNormal, "MirrorCleaned", "all mirrored secrets deleted")
		}
		return ctrl.Result{}, nil
	}

	// Ensure finalizer.
	if _, err := mirror.EnsureFinalizer(ctx, r.Client, src); err != nil {
		return ctrl.Result{}, fmt.Errorf("ensure finalizer on secret: %w", err)
	}

	// Compute target namespaces.
	targetNSNames, err := r.targetNamespaces(ctx,
		src.Namespace,
		src.Annotations[AnnotationNamespaceInclude],
		src.Annotations[AnnotationNamespaceExclude],
	)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Convergence.
	var failedNS []string
	for _, ns := range targetNSNames {
		if err := r.syncSecret(ctx, src, ns); err != nil {
			log.Error(err, "sync failed", "target", ns)
			failedNS = append(failedNS, ns)
		}
	}

	// Delete orphan mirrors (actual − desired).
	if err := r.deleteOrphanSecrets(ctx, src.Namespace, src.Name, toSet(targetNSNames)); err != nil {
		return ctrl.Result{}, err
	}

	if len(failedNS) > 0 {
		r.Recorder.Eventf(src, corev1.EventTypeWarning, "MirrorSyncFailed",
			"sync failed for namespaces: %s", strings.Join(failedNS, ", "))
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}
	r.Recorder.Eventf(src, corev1.EventTypeNormal, "MirrorSynced",
		"synced to %d namespace(s)", len(targetNSNames))
	return ctrl.Result{}, nil
}

func (r *MirrorReconciler) syncSecret(ctx context.Context, src *corev1.Secret, targetNS string) error {
	log := ctrl.LoggerFrom(ctx)
	key := types.NamespacedName{Namespace: targetNS, Name: src.Name}

	existing := &corev1.Secret{}
	err := r.Get(ctx, key, existing)
	if apierrors.IsNotFound(err) {
		obj := mirror.BuildMirrorSecret(src, targetNS)
		if err := r.Create(ctx, obj); err != nil {
			return fmt.Errorf("create mirror secret in %s: %w", targetNS, err)
		}
		log.Info("created mirror", "source", src.Namespace+"/"+src.Name, "target", targetNS)
		return nil
	}
	if err != nil {
		return fmt.Errorf("get mirror secret in %s: %w", targetNS, err)
	}

	if mirror.NeedsSyncSecret(src, existing) {
		log.Info("overwriting drifted resource", "namespace", targetNS, "name", src.Name)
		mirror.ApplyMirrorSecret(src, existing)
		if err := r.Update(ctx, existing); err != nil {
			return fmt.Errorf("update mirror secret in %s: %w", targetNS, err)
		}
		log.Info("updated mirror", "source", src.Namespace+"/"+src.Name, "target", targetNS)
	}
	return nil
}

func (r *MirrorReconciler) cleanupMirroredSecrets(ctx context.Context, srcNS, srcName string) error {
	list := &corev1.SecretList{}
	if err := r.List(ctx, list, client.MatchingLabels{
		LabelMirroredFrom: mirror.BuildMirroredFromValue(srcNS, srcName),
	}); err != nil {
		return fmt.Errorf("list mirrored secrets: %w", err)
	}
	for i := range list.Items {
		if err := r.Delete(ctx, &list.Items[i]); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("delete mirror secret %s/%s: %w",
				list.Items[i].Namespace, list.Items[i].Name, err)
		}
		ctrl.LoggerFrom(ctx).Info("deleted mirror",
			"source", srcNS+"/"+srcName,
			"target", list.Items[i].Namespace)
	}
	return nil
}

func (r *MirrorReconciler) deleteOrphanSecrets(ctx context.Context, srcNS, srcName string, desired map[string]struct{}) error {
	list := &corev1.SecretList{}
	if err := r.List(ctx, list, client.MatchingLabels{
		LabelMirroredFrom: mirror.BuildMirroredFromValue(srcNS, srcName),
	}); err != nil {
		return fmt.Errorf("list mirrored secrets for orphan check: %w", err)
	}
	for i := range list.Items {
		if _, ok := desired[list.Items[i].Namespace]; ok {
			continue
		}
		if err := r.Delete(ctx, &list.Items[i]); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("delete orphan secret %s/%s: %w",
				list.Items[i].Namespace, list.Items[i].Name, err)
		}
		ctrl.LoggerFrom(ctx).Info("deleted mirror",
			"source", srcNS+"/"+srcName,
			"target", list.Items[i].Namespace)
	}
	return nil
}

// ---- ConfigMap reconciliation --------------------------------------------

func (r *MirrorReconciler) reconcileConfigMap(ctx context.Context, src *corev1.ConfigMap) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx).WithValues("source", src.Namespace+"/"+src.Name)

	mirrorEnabled := src.Annotations[AnnotationAllowMirror] == "true"

	if !src.DeletionTimestamp.IsZero() || !mirrorEnabled {
		if err := r.cleanupMirroredConfigMaps(ctx, src.Namespace, src.Name); err != nil {
			return ctrl.Result{}, err
		}
		if err := mirror.RemoveFinalizer(ctx, r.Client, src); err != nil {
			return ctrl.Result{}, fmt.Errorf("remove finalizer from configmap: %w", err)
		}
		if mirrorEnabled {
			r.Recorder.Eventf(src, corev1.EventTypeNormal, "MirrorCleaned", "all mirrored configmaps deleted")
		}
		return ctrl.Result{}, nil
	}

	if _, err := mirror.EnsureFinalizer(ctx, r.Client, src); err != nil {
		return ctrl.Result{}, fmt.Errorf("ensure finalizer on configmap: %w", err)
	}

	targetNSNames, err := r.targetNamespaces(ctx,
		src.Namespace,
		src.Annotations[AnnotationNamespaceInclude],
		src.Annotations[AnnotationNamespaceExclude],
	)
	if err != nil {
		return ctrl.Result{}, err
	}

	var failedNS []string
	for _, ns := range targetNSNames {
		if err := r.syncConfigMap(ctx, src, ns); err != nil {
			log.Error(err, "sync failed", "target", ns)
			failedNS = append(failedNS, ns)
		}
	}

	if err := r.deleteOrphanConfigMaps(ctx, src.Namespace, src.Name, toSet(targetNSNames)); err != nil {
		return ctrl.Result{}, err
	}

	if len(failedNS) > 0 {
		r.Recorder.Eventf(src, corev1.EventTypeWarning, "MirrorSyncFailed",
			"sync failed for namespaces: %s", strings.Join(failedNS, ", "))
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}
	r.Recorder.Eventf(src, corev1.EventTypeNormal, "MirrorSynced",
		"synced to %d namespace(s)", len(targetNSNames))
	return ctrl.Result{}, nil
}

func (r *MirrorReconciler) syncConfigMap(ctx context.Context, src *corev1.ConfigMap, targetNS string) error {
	log := ctrl.LoggerFrom(ctx)
	key := types.NamespacedName{Namespace: targetNS, Name: src.Name}

	existing := &corev1.ConfigMap{}
	err := r.Get(ctx, key, existing)
	if apierrors.IsNotFound(err) {
		obj := mirror.BuildMirrorConfigMap(src, targetNS)
		if err := r.Create(ctx, obj); err != nil {
			return fmt.Errorf("create mirror configmap in %s: %w", targetNS, err)
		}
		log.Info("created mirror", "source", src.Namespace+"/"+src.Name, "target", targetNS)
		return nil
	}
	if err != nil {
		return fmt.Errorf("get mirror configmap in %s: %w", targetNS, err)
	}

	if mirror.NeedsSyncConfigMap(src, existing) {
		log.Info("overwriting drifted resource", "namespace", targetNS, "name", src.Name)
		mirror.ApplyMirrorConfigMap(src, existing)
		if err := r.Update(ctx, existing); err != nil {
			return fmt.Errorf("update mirror configmap in %s: %w", targetNS, err)
		}
		log.Info("updated mirror", "source", src.Namespace+"/"+src.Name, "target", targetNS)
	}
	return nil
}

func (r *MirrorReconciler) cleanupMirroredConfigMaps(ctx context.Context, srcNS, srcName string) error {
	list := &corev1.ConfigMapList{}
	if err := r.List(ctx, list, client.MatchingLabels{
		LabelMirroredFrom: mirror.BuildMirroredFromValue(srcNS, srcName),
	}); err != nil {
		return fmt.Errorf("list mirrored configmaps: %w", err)
	}
	for i := range list.Items {
		if err := r.Delete(ctx, &list.Items[i]); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("delete mirror configmap %s/%s: %w",
				list.Items[i].Namespace, list.Items[i].Name, err)
		}
		ctrl.LoggerFrom(ctx).Info("deleted mirror",
			"source", srcNS+"/"+srcName,
			"target", list.Items[i].Namespace)
	}
	return nil
}

func (r *MirrorReconciler) deleteOrphanConfigMaps(ctx context.Context, srcNS, srcName string, desired map[string]struct{}) error {
	list := &corev1.ConfigMapList{}
	if err := r.List(ctx, list, client.MatchingLabels{
		LabelMirroredFrom: mirror.BuildMirroredFromValue(srcNS, srcName),
	}); err != nil {
		return fmt.Errorf("list mirrored configmaps for orphan check: %w", err)
	}
	for i := range list.Items {
		if _, ok := desired[list.Items[i].Namespace]; ok {
			continue
		}
		if err := r.Delete(ctx, &list.Items[i]); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("delete orphan configmap %s/%s: %w",
				list.Items[i].Namespace, list.Items[i].Name, err)
		}
		ctrl.LoggerFrom(ctx).Info("deleted mirror",
			"source", srcNS+"/"+srcName,
			"target", list.Items[i].Namespace)
	}
	return nil
}

// ---- Shared helpers -------------------------------------------------------

// targetNamespaces lists all Active namespaces and applies include/exclude filters.
func (r *MirrorReconciler) targetNamespaces(ctx context.Context, sourceNS, includeRaw, excludeRaw string) ([]string, error) {
	nsList := &corev1.NamespaceList{}
	if err := r.List(ctx, nsList); err != nil {
		return nil, fmt.Errorf("list namespaces: %w", err)
	}

	includePatterns := splitCSV(includeRaw)
	excludePatterns := splitCSV(excludeRaw)

	return mirror.MatchNamespaces(nsList.Items, sourceNS, includePatterns, excludePatterns), nil
}

func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

func toSet(names []string) map[string]struct{} {
	s := make(map[string]struct{}, len(names))
	for _, n := range names {
		s[n] = struct{}{}
	}
	return s
}

// ---- Predicates -----------------------------------------------------------

// allowMirrorPredicate passes only objects with allow-mirror=true annotation.
var allowMirrorPredicate = predicate.NewPredicateFuncs(func(obj client.Object) bool {
	return obj.GetAnnotations()[AnnotationAllowMirror] == "true"
})

// mirroredResourcePredicate passes only objects that carry the LabelMirroredFrom label.
var mirroredResourcePredicate = predicate.NewPredicateFuncs(func(obj client.Object) bool {
	_, ok := obj.GetLabels()[LabelMirroredFrom]
	return ok
})

// sourceFromMirrorMapFunc maps a mirrored resource back to its source's reconcile request.
func sourceFromMirrorMapFunc(_ context.Context, obj client.Object) []reconcile.Request {
	annotations := obj.GetAnnotations()
	srcNS := annotations[AnnotationSourceNamespace]
	srcName := annotations[AnnotationSourceName]
	if srcNS == "" || srcName == "" {
		return nil
	}
	return []reconcile.Request{{
		NamespacedName: types.NamespacedName{Namespace: srcNS, Name: srcName},
	}}
}

// ---- SetupWithManager -----------------------------------------------------

func (r *MirrorReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Allow-mirror predicate: only enqueue source resources with the annotation.
	sourcePredicate := builder.WithPredicates(
		allowMirrorPredicate,
		predicate.Or[client.Object](
			predicate.GenerationChangedPredicate{},
			predicate.AnnotationChangedPredicate{},
		),
	)

	// Drift detection predicate for mirrored resources.
	driftPredicate := builder.WithPredicates(
		mirroredResourcePredicate,
		predicate.Or[client.Object](
			predicate.ResourceVersionChangedPredicate{},
			predicate.Funcs{DeleteFunc: func(event.DeleteEvent) bool { return true }},
		),
	)

	mirrorMapHandler := handler.EnqueueRequestsFromMapFunc(sourceFromMirrorMapFunc)

	// Build the namespace-to-sources MapFunc for new-namespace fan-out.
	nsMapFunc := NewNamespaceToSourcesMapFunc(mgr.GetClient())

	return ctrl.NewControllerManagedBy(mgr).
		Named("mirror").
		WithOptions(controller.Options{MaxConcurrentReconciles: 3}).
		// Watch source Secrets.
		For(&corev1.Secret{}, sourcePredicate).
		// Watch source ConfigMaps.
		Watches(&corev1.ConfigMap{},
			&handler.EnqueueRequestForObject{},
			builder.WithPredicates(
				allowMirrorPredicate,
				predicate.Or[client.Object](
					predicate.GenerationChangedPredicate{},
					predicate.AnnotationChangedPredicate{},
				),
			),
		).
		// Drift detection: mirrored Secrets → enqueue source Secret.
		Watches(&corev1.Secret{}, mirrorMapHandler, driftPredicate).
		// Drift detection: mirrored ConfigMaps → enqueue source ConfigMap.
		Watches(&corev1.ConfigMap{}, mirrorMapHandler, driftPredicate).
		// New Namespace → fan out to all mirroring-enabled sources.
		Watches(&corev1.Namespace{},
			handler.EnqueueRequestsFromMapFunc(nsMapFunc),
			builder.WithPredicates(namespaceBecameActivePredicate),
		).
		Complete(r)
}
