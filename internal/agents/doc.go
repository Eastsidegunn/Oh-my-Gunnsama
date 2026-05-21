// Package agents holds code-defined agent registrations. Each file in this
// package defines one agent via an init() function that calls
// registry.Register. The package is meant to be blank-imported by cmd/omg
// so the agents are available at daemon startup.
package agents
