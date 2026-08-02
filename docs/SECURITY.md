# Security model

`sudoconsole` is a security-sensitive tool: it captures the sudo
password on behalf of a CLI agent and forwards it to `sudo(8)`. This
document captures the threat model, the guarantees we provide, and
the mitigations in place.

## Threat model

| Adversary | Capability | Mitigation |
|-----------|-----------|------------|
| Co-resident agent (e.g. another AI agent in the same session) | Read `/proc/<pid>/cmdline`, `/proc/<pid>/environ`, run `ps`, capture audit logs | Secret is fed via PTY stdin, never argv; audit logs redact `--password=` / `password=` substrings; coredumps disabled (RLIMIT_CORE=0); see below. |
| Local user with shell access | Read coredump files, inspect environment of running processes | RLIMIT_CORE=0 on the parent process (inherited by every child); PTY-allocated stdin (no `ps` leak); secret zeroised after use. |
| `ptrace` attacker (same uid) | Attach to the running process | We do not defend against same-uid ptrace. Document that running sudoconsole under a debugger defeats all in-memory secrecy guarantees. |
| Filesystem observer | Read audit log, config files, journalctl | Audit log is JSONL with secrets redacted; config never holds the password; journalctl grep test added in M12. |
| Side-channel (timing) | Measure duration of `ConstantTimeEquals` | We use `crypto/subtle.ConstantTimeCompare` for credential checks. |

## Defense in depth — what the binary does

1. **PTY-only secret transport.** The password is fed via the
   master fd of an allocated pseudoterminal. `exec.Command` is
   configured with `argv` that contains `sudo`, `-S`, `-p`, `""`,
   `-v` — **no password bytes**. A sibling process that runs `ps`
   or reads `/proc/<pid>/cmdline` sees only the argv, never the
   secret.
2. **RLIMIT_CORE=0 on the parent.** Before the sudo child is
   forked, the auth gateway sets `RLIMIT_CORE=0` via `prlimit(2)`
   on the calling process and clears `PR_SET_DUMPABLE`. The limit
   is inherited across `execve`, so the child (and any
   descendants) cannot coredump either. Verified by the
   `TestDisableCoreDump_RlimitZeroed` unit test.
3. **Secret lifetime is bounded.** The `domain.Credential` type
   wraps a heap-allocated `[]byte` that is overwritten with zeros
   when `Zeroize` is called. `Bytes()` returns a *defensive copy*
   so callers cannot accidentally prolong the lifetime of the
   secret through a slice they store. Verified by
   `TestCredential_ZeroizeOverwritesBackingArray` and
   `TestCredential_BytesIsSeparateAllocation`.
4. **Audit log redaction.** The audit logger scans every event for
   substring matches of `--password=`, `--passwd=`, `--token=`,
   `--secret=`, `password=`, `passwd=`, `token=`, `secret=` and
   marks the event as `[redacted]` if any are found. (See
   `internal/infrastructure/audit/audit.go:281-292`.)
5. **Constant-time comparisons.** All credential equality checks
   route through `subtle.ConstantTimeCompare`. See
   `Credential.ConstantTimeEquals`.
6. **TOML pattern validation runs at config load** so a bad glob
   or ReDoS-prone regex never reaches the runtime matcher. The
   `policy.ValidateEach` helper compiles every pattern in isolation
   and returns a per-pattern error slice; the loader rejects
   non-regex-safe patterns before they can be matched against
   arbitrary command lines.
7. **Pattern fuzzing.** `go test -fuzz=FuzzMatcher` /
   `go test -fuzz=FuzzValidateEach` /
   `go test -fuzz=FuzzLoader` exercise the matcher and config
   loader with random inputs. Each test was run for ≥10 seconds
   (≈700k–1.4M iterations) without a panic.

## Manual pen-test (M12)

`TestPassword_NotLeakedToArgvOrEnv` exercises the gateway with a
unique sentinel secret (`P4NTHERE-pen-test-2026-m12`), waits for
sudo to exit, and then sweeps:

- `/proc/self/cmdline`
- `/proc/self/environ`
- `journalctl -n 100`

and asserts the sentinel appears in none of them. The test
self-skips vectors that are inaccessible (e.g. journalctl on
hosts without log access).

## Operational guidance

- Do **not** run `sudoconsole` under a debugger or `strace`. The
  ptrace-attached process can read any memory, including the
  secret while it sits in the PTY write buffer. The
  `RequireTTY` config option (default `true`) refuses to run if
  the stdin is not a TTY.
- The sudo timestamp lives at `/var/db/sudo/ts/<base64(username)>`
  on Linux. If a co-resident user can read that directory, they
  can replay the cached credential. Restrict the directory with
  `chmod 700 /var/db/sudo/ts` if your environment permits.
- The audit log file is created with mode `0600`. Verify the
  containing directory also restricts group/other access.

## Reporting vulnerabilities

Please open a private security advisory on GitHub (Security tab →
"Report a vulnerability"). Do not file public issues for security
bugs.