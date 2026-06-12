// Package all blank-imports every source so a single import wires their
// init()-time registration into the registry.
package all

import (
	_ "housing-waitlist/internal/source/kkik"
	_ "housing-waitlist/internal/source/sdk"
)
