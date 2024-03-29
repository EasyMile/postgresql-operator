package utils

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"github.com/easymile/postgresql-operator/api/postgresql/common"
	postgresqlv1alpha1 "github.com/easymile/postgresql-operator/api/postgresql/v1alpha1"
	"github.com/easymile/postgresql-operator/internal/controller/postgresql/postgres"
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
	return hex.EncodeToString(sha256Bytes), nil
}

func CreatePgInstance(
	reqLogger logr.Logger,
	secretData map[string][]byte,
	pgec *postgresqlv1alpha1.PostgresqlEngineConfiguration,
) postgres.PG {
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

func GetSecret(ctx context.Context, cl client.Client, name, namespace string) (*corev1.Secret, error) {
	secret := &corev1.Secret{}
	err := cl.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, secret)

	return secret, err
}

func FindSecretPgEngineCfg(
	ctx context.Context,
	cl client.Client,
	instance *postgresqlv1alpha1.PostgresqlEngineConfiguration,
) (*corev1.Secret, error) {
	secret := &corev1.Secret{}
	err := cl.Get(ctx, types.NamespacedName{Name: instance.Spec.SecretName, Namespace: instance.Namespace}, secret)

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

func CreateNameKey(name, namespace, instanceNamespace string) string {
	res := ""

	if namespace != "" {
		res += namespace
	} else {
		res += instanceNamespace
	}

	res += "/" + name

	return res
}

func CreateNameKeyForSavedPools(pgecName, pgecNamespace string) string {
	return pgecNamespace + "/" + pgecName
}

func FindPgEngineCfg(
	ctx context.Context,
	cl client.Client,
	instance *postgresqlv1alpha1.PostgresqlDatabase,
) (*postgresqlv1alpha1.PostgresqlEngineConfiguration, error) {
	// Try to get namespace from spec
	namespace := instance.Spec.EngineConfiguration.Namespace
	if namespace == "" {
		// Namespace not found, take it from instance namespace
		namespace = instance.Namespace
	}

	pgEngineCfg := &postgresqlv1alpha1.PostgresqlEngineConfiguration{}
	err := cl.Get(ctx, client.ObjectKey{
		Name:      instance.Spec.EngineConfiguration.Name,
		Namespace: namespace,
	}, pgEngineCfg)

	return pgEngineCfg, err
}

func FindPgDatabaseFromLink(
	ctx context.Context,
	cl client.Client,
	link *common.CRLink,
	instanceNamespace string,
) (*postgresqlv1alpha1.PostgresqlDatabase, error) {
	// Try to get namespace from spec
	namespace := link.Namespace
	if namespace == "" {
		// Namespace not found, take it from instance namespace
		namespace = instanceNamespace
	}

	pgDatabase := &postgresqlv1alpha1.PostgresqlDatabase{}
	err := cl.Get(ctx, client.ObjectKey{
		Name:      link.Name,
		Namespace: namespace,
	}, pgDatabase)

	return pgDatabase, err
}
