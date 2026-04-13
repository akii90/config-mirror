/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"flag"
	"github.com/akii90/config-mirror/internal/controller"
	"os"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"time"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
}

// nolint:gocyclo
func main() {
	opts := zap.Options{Development: true}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	// SyncPeriod=0 disables the informer's periodic resync. Drift is detected
	// reactively by watching mirrored (target) resources instead.
	noResync := time.Duration(0)

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
		// level-driven, disable resync
		Cache: cache.Options{
			SyncPeriod: &noResync,
		},
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	// Set up field indexers required by NamespaceReconciler fan-out queries.
	nsWatcher := &controller.NamespaceReconciler{Client: mgr.GetClient()}
	if err := nsWatcher.SetupIndexers(mgr); err != nil {
		setupLog.Error(err, "unable to set up field indexers")
		os.Exit(1)
	}

	if err := (&controller.MirrorReconciler{
		Client: mgr.GetClient(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create MirrorReconciler")
		os.Exit(1)
	}

	if err := nsWatcher.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create NamespaceReconciler")
		os.Exit(1)
	}

	setupLog.Info("starting config-mirror manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
