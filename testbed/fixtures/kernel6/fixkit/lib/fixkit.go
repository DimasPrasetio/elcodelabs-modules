// Package fixkit is a testbed fixture: shared Go code a module imports as a Go kit.
package fixkit

import "fmt"

// OrderNumber is a document number for id.
func OrderNumber(id int64) string { return fmt.Sprintf("ORD-%06d", id) }
