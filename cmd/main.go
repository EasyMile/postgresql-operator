/*
Copyright 2022.

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
	"fmt"
	"os"
	"time"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/metrics"

	postgresqlv1alpha1 "github.com/easymile/postgresql-operator/api/postgresql/v1alpha1"
	postgresqlcontrollers "github.com/easymile/postgresql-operator/internal/controller/postgresql"
	"github.com/prometheus/client_golang/prometheus"
	//+kubebuilder:scaffold:imports
)

var (
	scheme                              = runtime.NewScheme()
	setupLog                            = ctrl.Log.WithName("setup")
	controllerRuntimeDetailedErrorTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "controller_runtime_reconcile_detailed_errors_total",
			Help: "Total number of reconciliation errors per controller detailed with resource namespace and name.",
		},
		[]string{"controller", "namespace", "name"},
	)
)

func init() {
	// Register custom metrics with the global prometheus registry
	metrics.Registry.MustRegister(controllerRuntimeDetailedErrorTotal)

	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(postgresqlv1alpha1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
} //nolint: wsl // Needed by operator

func main() {
	var metricsAddr, probeAddr, resyncPeriodStr string

	var enableLeaderElection bool

	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.StringVar(&resyncPeriodStr, "resync-period", "30s", "The resync period to reload all resources for auto-heal procedures.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")

	opts := zap.Options{
		Development: false,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	// Parse duration
	resyncPeriod, err := time.ParseDuration(resyncPeriodStr)
	// Check error
	if err != nil {
		setupLog.Error(err, "unable to parse resync period")
		os.Exit(1)
	}
	// Log
	setupLog.Info(fmt.Sprintf("Starting manager with %s resync period", resyncPeriodStr))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     metricsAddr,
		Port:                   9443, //nolint: gomnd // Because generated
		HealthProbeBindAddress: probeAddr,
		SyncPeriod:             &resyncPeriod,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "07c031df.easymile.com",
		// LeaderElectionReleaseOnCancel defines if the leader should step down voluntarily
		// when the Manager ends. This requires the binary to immediately end when the
		// Manager is stopped, otherwise, this setting is unsafe. Setting this significantly
		// speeds up voluntary leader transitions as the new leader don't have to wait
		// LeaseDuration time first.
		//
		// In the default scaffold provided, the program ends immediately after
		// the manager stops, so would be fine to enable this option. However,
		// if you are doing or is intended to do any operation such as perform cleanups
		// after the manager stops then its usage might be unsafe.
		// LeaderElectionReleaseOnCancel: true,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if err = (&postgresqlcontrollers.PostgresqlEngineConfigurationReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		Recorder: mgr.GetEventRecorderFor("postgresqlengineconfiguration-controller"),
		Log: ctrl.Log.WithValues(
			"controller",
			"postgresqlengineconfiguration",
			"controllerKind",
			"PostgresqlEngineConfiguration",
			"controllerGroup",
			"postgresql.easymile.com",
		),
		ControllerRuntimeDetailedErrorTotal: controllerRuntimeDetailedErrorTotal,
		ControllerName:                      "postgresqlengineconfiguration",
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "PostgresqlEngineConfiguration")
		os.Exit(1)
	}

	if err = (&postgresqlcontrollers.PostgresqlDatabaseReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		Recorder: mgr.GetEventRecorderFor("postgresqldatabase-controller"),
		Log: ctrl.Log.WithValues(
			"controller",
			"postgresqldatabase",
			"controllerKind",
			"PostgresqlDatabase",
			"controllerGroup",
			"postgresql.easymile.com",
		),
		ControllerRuntimeDetailedErrorTotal: controllerRuntimeDetailedErrorTotal,
		ControllerName:                      "postgresqldatabase",
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "PostgresqlDatabase")
		os.Exit(1)
	}

	if err = (&postgresqlcontrollers.PostgresqlUserRoleReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		Recorder: mgr.GetEventRecorderFor("postgresqluserrole-controller"),
		Log: ctrl.Log.WithValues(
			"controller",
			"postgresqluserrole",
			"controllerKind",
			"PostgresqlUserRole",
			"controllerGroup",
			"postgresql.easymile.com",
		),
		ControllerRuntimeDetailedErrorTotal: controllerRuntimeDetailedErrorTotal,
		ControllerName:                      "postgresqluserrole",
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "PostgresqlUserRole")
		os.Exit(1)
	}
	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}

	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")

	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
