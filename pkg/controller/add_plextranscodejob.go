package controller

import (
	"github.com/mcadam/plex-operator/pkg/controller/plextranscodejob"
)

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, plextranscodejob.Add)
}
