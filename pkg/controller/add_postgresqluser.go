package controller

import (
	"github.com/easymile/postgresql-operator/pkg/controller/postgresqluser"
)

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, postgresqluser.Add)
}
