// Package usecase contains application-layer use cases.
//
// Use cases depend only on the domain package and orchestrate business logic
// via the ports declared in domain. They do not know about concrete adapters
// (PTY, sudo, etc.) — those are wired in the transport/cli composition root.
package usecase
