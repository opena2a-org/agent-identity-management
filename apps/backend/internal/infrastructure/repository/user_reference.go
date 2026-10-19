package repository

import "github.com/google/uuid"

// userRef is the value written to a column that references users(id). uuid.Nil
// names no user and is written as NULL: no user has the zero id, so writing it
// would fail the foreign key. An agent whose creator was deleted reads back with
// CreatedBy uuid.Nil (migration 135 sets agents.created_by to NULL on the delete),
// and the handlers pass an agent's CreatedBy as the user its requests act for, so
// its capability requests, its MCP servers and its audit rows reach the
// repositories with uuid.Nil.
func userRef(id uuid.UUID) uuid.NullUUID {
	return uuid.NullUUID{UUID: id, Valid: id != uuid.Nil}
}

// userRefPtr is userRef for a reference held as a pointer; nil also names no
// user.
func userRefPtr(id *uuid.UUID) uuid.NullUUID {
	if id == nil {
		return uuid.NullUUID{}
	}
	return userRef(*id)
}
