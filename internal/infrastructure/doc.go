// Package infrastructure contains adapters that implement the ports
// declared in internal/domain.
//
// Each subpackage wraps a single external concern (PTY, sudo, cache,
// config, policy, audit, agent) so that swapping one implementation
// for another requires changing exactly one subpackage.
package infrastructure
