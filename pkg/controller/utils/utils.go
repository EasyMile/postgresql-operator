package utils

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"

	postgresqlv1alpha1 "github.com/easymile/postgresql-operator/pkg/apis/postgresql/v1alpha1"
	"github.com/easymile/postgresql-operator/pkg/postgres"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func CalculateHash(spec interface{}) (string, error) {
	// Json marshal spec
	bytes, err := json.Marshal(spec)
	if err != nil {
		return "", err
	}
	// Sha on bytes array
	sha256Res := sha256.Sum256(bytes)
	sha256Bytes := sha256Res[:]
	// Transform it to string
	return fmt.Sprintf("%x", sha256Bytes), nil
}

func CreatePgInstance(reqLogger logr.Logger, secretData map[string][]byte, pgec *postgresqlv1alpha1.PostgresqlEngineConfiguration) postgres.PG {
	spec := pgec.Spec
	user := string(secretData["user"])
	password := string(secretData["password"])

	return postgres.NewPG(
		CreateNameKeyForSavedPools(pgec.Name, pgec.Namespace),
		spec.Host,
		user,
		password,
		spec.URIArgs,
		spec.DefaultDatabase,
		spec.Port,
		spec.Provider,
		reqLogger,
	)
}

func FindSecretPgEngineCfg(cl client.Client, instance *postgresqlv1alpha1.PostgresqlEngineConfiguration) (*corev1.Secret, error) {
	secret := &corev1.Secret{}
	err := cl.Get(context.TODO(), types.NamespacedName{Name: instance.Spec.SecretName, Namespace: instance.Namespace}, secret)
	return secret, err
}

func CloseDatabaseSavedPoolsForName(instance *postgresqlv1alpha1.PostgresqlDatabase, database string) error {
	// Try to get namespace from spec
	namespace := instance.Spec.EngineConfiguration.Namespace
	if namespace == "" {
		// Namespace not found, take it from instance namespace
		namespace = instance.Namespace
	}

	return postgres.CloseDatabaseSavedPoolsForName(
		CreateNameKeyForSavedPools(instance.Spec.EngineConfiguration.Name, namespace),
		database,
	)
}

func CreateNameKeyForSavedPools(pgecName, pgecNamespace string) string {
	return pgecNamespace + "/" + pgecName
}

func FindPgEngineCfg(cl client.Client, instance *postgresqlv1alpha1.PostgresqlDatabase) (*postgresqlv1alpha1.PostgresqlEngineConfiguration, error) {
	// Try to get namespace from spec
	namespace := instance.Spec.EngineConfiguration.Namespace
	if namespace == "" {
		// Namespace not found, take it from instance namespace
		namespace = instance.Namespace
	}

	pgEngineCfg := &postgresqlv1alpha1.PostgresqlEngineConfiguration{}
	err := cl.Get(context.TODO(), client.ObjectKey{
		Name:      instance.Spec.EngineConfiguration.Name,
		Namespace: namespace,
	}, pgEngineCfg)

	return pgEngineCfg, err
}

func FindPgDatabase(cl client.Client, instance *postgresqlv1alpha1.PostgresqlUser) (*postgresqlv1alpha1.PostgresqlDatabase, error) {
	// Try to get namespace from spec
	namespace := instance.Spec.Database.Namespace
	if namespace == "" {
		// Namespace not found, take it from instance namespace
		namespace = instance.Namespace
	}

	pgDatabase := &postgresqlv1alpha1.PostgresqlDatabase{}
	err := cl.Get(context.TODO(), client.ObjectKey{
		Name:      instance.Spec.Database.Name,
		Namespace: namespace,
	}, pgDatabase)

	return pgDatabase, err
}
