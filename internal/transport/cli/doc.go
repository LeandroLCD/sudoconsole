// Package cli contains the cobra commands that form the user-facing
// surface of sudoconsole.
//
// This is the composition root: it builds the concrete infrastructure
// adapters, injects them into the use cases, and wires the resulting
// commands together.
package cli
