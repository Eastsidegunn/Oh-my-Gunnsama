// Package opencode documents the OpenCode adapter boundary.
//
// The OpenCode JS bridge does not implement internal/adapter.HostAdapter in Go.
// It follows OMG's v1 wire protocol and the same behavioral contract: translate
// OpenCode callback input into protocol.Request, forward it to OMG, and apply
// protocol.Response to OpenCode output without owning routing, prompt building,
// fallback/retry, state transitions, or guard policy.
package opencode
