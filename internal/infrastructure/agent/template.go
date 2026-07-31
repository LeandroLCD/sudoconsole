package agent

import (
	"github.com/LeandroLCD/sudoconsole/internal/domain"
)

// commandsMarkdown returns the slash-command markdown that the user
// invokes from inside the agent to run a privileged command. The same
// text is used by every adapter; adapters just choose where to write
// it.
//
// The marker is the same canonical `# sudoconsole-marker: <kind>`
// string used by guardedInstall so idempotency checks work uniformly
// across adapters.
func commandsMarkdown(kind domain.AgentKind) []byte {
	return []byte(`# ` + marker(kind) + ` managed by sudoconsole. do not edit by hand.
---
description: Run a shell command under sudo with policy enforcement.
---

# sudoconsole

Usage:

  /sudoconsole exec <cmd...>

Examples:

  /sudoconsole exec apt update
  /sudoconsole check
  /sudoconsole auth
`)
}

// adapterCmd returns the adapter command that the agent invokes.
// For most agents this is just "sudoconsole"; we centralise the
// constant so it stays consistent.
func adapterCmd() string {
	return "sudoconsole"
}
