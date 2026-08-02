#!/bin/sh
# Post-install: drop a sample sudoers fragment so the user
# understands the privileged operations required. The fragment
# is *not* enabled by default.
if [ ! -f /etc/sudoers.d/sudoconsole ]; then
    cat > /etc/sudoers.d/sudoconsole.example <<SUDOERS_EOF
# Example: allow the sudoconsole group to refresh the sudo
# timestamp without a password. Enable manually after review.
# %sudoconsole ALL=(ALL) NOPASSWD: /usr/bin/sudo -v
SUDOERS_EOF
    chmod 0440 /etc/sudoers.d/sudoconsole.example
fi