// Package schema provides the fresh database definition for isolated test fixtures.
package schema

import _ "embed"

// InitSQL is executed by tests in a new schema; the server does not apply DDL.
//
//go:embed init.sql
var InitSQL string
