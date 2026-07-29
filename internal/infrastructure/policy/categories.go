package policy

import "github.com/LeandroLCD/sudoconsole/internal/domain"

// CategoryRegistry maps binary basenames to their risk classification.
//
// The map is intentionally curated (rather than learned) because policy
// decisions are safety-critical; a single missing category is a security
// hole. Additions should be reviewed alongside the policy defaults in
// domain.DefaultPolicy.
//
// Categories not listed here are categorized as CategoryUnknown with
// RiskMedium (treated as "be careful, but not blocked by default").
type CategoryRegistry struct {
	entries map[string]domain.CategoryEntry
}

// DefaultCategoryRegistry returns the built-in registry.
func DefaultCategoryRegistry() *CategoryRegistry {
	r := &CategoryRegistry{entries: make(map[string]domain.CategoryEntry, len(builtinCategories))}
	for k, v := range builtinCategories {
		r.entries[k] = v
	}
	return r
}

// Lookup returns the category entry for the given binary basename.
// Returns (zero entry, false) if not found.
func (r *CategoryRegistry) Lookup(binary string) (domain.CategoryEntry, bool) {
	if r == nil {
		return domain.CategoryEntry{}, false
	}
	e, ok := r.entries[binary]
	return e, ok
}

// Add inserts or overrides an entry. Returns the previous value (if any).
func (r *CategoryRegistry) Add(binary string, category domain.Category, risk domain.Risk, notes string) (prev domain.CategoryEntry, hadPrev bool) {
	if r == nil {
		return domain.CategoryEntry{}, false
	}
	if r.entries == nil {
		r.entries = make(map[string]domain.CategoryEntry)
	}
	prev, hadPrev = r.entries[binary]
	r.entries[binary] = domain.CategoryEntry{
		Binary:    binary,
		Category:  category,
		Risk:      risk,
		Notes:     notes,
		BuiltIn:   false,
		Overrides: hadPrev,
	}
	return prev, hadPrev
}

// Remove deletes an entry. Returns true if it existed.
func (r *CategoryRegistry) Remove(binary string) bool {
	if r == nil || r.entries == nil {
		return false
	}
	_, ok := r.entries[binary]
	delete(r.entries, binary)
	return ok
}

// All returns a stable snapshot of all entries (sorted by binary name would
// be nicer, but the order doesn't matter for policy evaluation).
func (r *CategoryRegistry) All() []domain.CategoryEntry {
	if r == nil || r.entries == nil {
		return nil
	}
	out := make([]domain.CategoryEntry, 0, len(r.entries))
	for _, e := range r.entries {
		out = append(out, e)
	}
	return out
}

// Len reports how many entries are registered.
func (r *CategoryRegistry) Len() int {
	if r == nil {
		return 0
	}
	return len(r.entries)
}

// builtinCategories is the curated table shipped with sudoconsole.
//
// Risk levels:
//   - Critical: blocks by default (ssh, nc, bash, passwd, visudo, etc.)
//   - High:     blocks by default in strict policy (useradd, crontab)
//   - Medium:   not blocked by default but flagged (apt, systemctl, curl)
//   - Low:      routine utilities
var builtinCategories = map[string]domain.CategoryEntry{
	// --- Remote access ---
	"ssh":    {Binary: "ssh", Category: domain.CategoryRemoteAccess, Risk: domain.RiskCritical, Notes: "Secure shell remote login", BuiltIn: true},
	"scp":    {Binary: "scp", Category: domain.CategoryRemoteAccess, Risk: domain.RiskCritical, Notes: "Secure copy over SSH", BuiltIn: true},
	"sftp":   {Binary: "sftp", Category: domain.CategoryRemoteAccess, Risk: domain.RiskCritical, Notes: "SSH file transfer", BuiltIn: true},
	"rsync":  {Binary: "rsync", Category: domain.CategoryRemoteAccess, Risk: domain.RiskHigh, Notes: "File sync (can use SSH)", BuiltIn: true},
	"mosh":   {Binary: "mosh", Category: domain.CategoryRemoteAccess, Risk: domain.RiskCritical, Notes: "Mobile shell", BuiltIn: true},
	"telnet": {Binary: "telnet", Category: domain.CategoryRemoteAccess, Risk: domain.RiskCritical, Notes: "Unencrypted remote shell", BuiltIn: true},
	"rclone": {Binary: "rclone", Category: domain.CategoryRemoteAccess, Risk: domain.RiskHigh, Notes: "Cloud sync", BuiltIn: true},
	"sshfs":  {Binary: "sshfs", Category: domain.CategoryRemoteAccess, Risk: domain.RiskHigh, Notes: "Filesystem over SSH", BuiltIn: true},
	"ftp":    {Binary: "ftp", Category: domain.CategoryRemoteAccess, Risk: domain.RiskHigh, Notes: "Unencrypted file transfer", BuiltIn: true},
	"curl":   {Binary: "curl", Category: domain.CategoryRemoteAccess, Risk: domain.RiskMedium, Notes: "Can exfiltrate data", BuiltIn: true},
	"wget":   {Binary: "wget", Category: domain.CategoryRemoteAccess, Risk: domain.RiskMedium, Notes: "Can exfiltrate data", BuiltIn: true},

	// --- Network tooling / reverse shells ---
	"nc":     {Binary: "nc", Category: domain.CategoryRemoteAccess, Risk: domain.RiskCritical, Notes: "Netcat — reverse shells", BuiltIn: true},
	"ncat":   {Binary: "ncat", Category: domain.CategoryRemoteAccess, Risk: domain.RiskCritical, Notes: "Nmap netcat", BuiltIn: true},
	"socat":  {Binary: "socat", Category: domain.CategoryRemoteAccess, Risk: domain.RiskCritical, Notes: "SOcket CAT — exec: shells", BuiltIn: true},
	"netcat": {Binary: "netcat", Category: domain.CategoryRemoteAccess, Risk: domain.RiskCritical, Notes: "Netcat (alt)", BuiltIn: true},

	// --- Credential exposure ---
	"ssh-keygen": {Binary: "ssh-keygen", Category: domain.CategoryCredentialExposure, Risk: domain.RiskCritical, Notes: "Generates/exports SSH keys", BuiltIn: true},
	"visudo":     {Binary: "visudo", Category: domain.CategoryCredentialExposure, Risk: domain.RiskCritical, Notes: "Edits sudoers file", BuiltIn: true},
	"vipw":       {Binary: "vipw", Category: domain.CategoryCredentialExposure, Risk: domain.RiskCritical, Notes: "Edits /etc/passwd", BuiltIn: true},
	"vigr":       {Binary: "vigr", Category: domain.CategoryCredentialExposure, Risk: domain.RiskCritical, Notes: "Edits /etc/group", BuiltIn: true},
	"chage":      {Binary: "chage", Category: domain.CategoryCredentialExposure, Risk: domain.RiskHigh, Notes: "Modifies password aging", BuiltIn: true},
	"gpg":        {Binary: "gpg", Category: domain.CategoryCredentialExposure, Risk: domain.RiskHigh, Notes: "May export secret keys", BuiltIn: true},
	"openssl":    {Binary: "openssl", Category: domain.CategoryCredentialExposure, Risk: domain.RiskHigh, Notes: "Can generate/export keys", BuiltIn: true},
	"vault":      {Binary: "vault", Category: domain.CategoryCredentialExposure, Risk: domain.RiskHigh, Notes: "HashiCorp Vault CLI", BuiltIn: true},
	"op":         {Binary: "op", Category: domain.CategoryCredentialExposure, Risk: domain.RiskHigh, Notes: "1Password CLI", BuiltIn: true},

	// --- User mgmt / credential change ---
	"passwd":   {Binary: "passwd", Category: domain.CategoryUserManagement, Risk: domain.RiskHigh, Notes: "Changes passwords", BuiltIn: true},
	"chpasswd": {Binary: "chpasswd", Category: domain.CategoryUserManagement, Risk: domain.RiskHigh, Notes: "Bulk password change", BuiltIn: true},
	"useradd":  {Binary: "useradd", Category: domain.CategoryUserManagement, Risk: domain.RiskHigh, Notes: "Creates users", BuiltIn: true},
	"userdel":  {Binary: "userdel", Category: domain.CategoryUserManagement, Risk: domain.RiskHigh, Notes: "Deletes users", BuiltIn: true},
	"usermod":  {Binary: "usermod", Category: domain.CategoryUserManagement, Risk: domain.RiskHigh, Notes: "Modifies users", BuiltIn: true},
	"groupadd": {Binary: "groupadd", Category: domain.CategoryUserManagement, Risk: domain.RiskHigh, Notes: "Creates groups", BuiltIn: true},
	"groupmod": {Binary: "groupmod", Category: domain.CategoryUserManagement, Risk: domain.RiskHigh, Notes: "Modifies groups", BuiltIn: true},
	"groupdel": {Binary: "groupdel", Category: domain.CategoryUserManagement, Risk: domain.RiskHigh, Notes: "Deletes groups", BuiltIn: true},
	"chsh":     {Binary: "chsh", Category: domain.CategoryUserManagement, Risk: domain.RiskHigh, Notes: "Changes login shell", BuiltIn: true},
	"chfn":     {Binary: "chfn", Category: domain.CategoryUserManagement, Risk: domain.RiskHigh, Notes: "Changes user info", BuiltIn: true},

	// --- Shell spawn (interpreters / shells) ---
	"bash":    {Binary: "bash", Category: domain.CategoryShellSpawn, Risk: domain.RiskCritical, Notes: "Interactive shell", BuiltIn: true},
	"sh":      {Binary: "sh", Category: domain.CategoryShellSpawn, Risk: domain.RiskCritical, Notes: "POSIX shell", BuiltIn: true},
	"zsh":     {Binary: "zsh", Category: domain.CategoryShellSpawn, Risk: domain.RiskCritical, Notes: "Z shell", BuiltIn: true},
	"fish":    {Binary: "fish", Category: domain.CategoryShellSpawn, Risk: domain.RiskHigh, Notes: "Friendly shell", BuiltIn: true},
	"csh":     {Binary: "csh", Category: domain.CategoryShellSpawn, Risk: domain.RiskHigh, Notes: "C shell", BuiltIn: true},
	"tcsh":    {Binary: "tcsh", Category: domain.CategoryShellSpawn, Risk: domain.RiskHigh, Notes: "TENEX C shell", BuiltIn: true},
	"dash":    {Binary: "dash", Category: domain.CategoryShellSpawn, Risk: domain.RiskHigh, Notes: "Debian Almquist shell", BuiltIn: true},
	"ksh":     {Binary: "ksh", Category: domain.CategoryShellSpawn, Risk: domain.RiskHigh, Notes: "Korn shell", BuiltIn: true},
	"python":  {Binary: "python", Category: domain.CategoryShellSpawn, Risk: domain.RiskHigh, Notes: "Python (can spawn shell)", BuiltIn: true},
	"python3": {Binary: "python3", Category: domain.CategoryShellSpawn, Risk: domain.RiskHigh, Notes: "Python 3", BuiltIn: true},
	"python2": {Binary: "python2", Category: domain.CategoryShellSpawn, Risk: domain.RiskHigh, Notes: "Python 2", BuiltIn: true},
	"perl":    {Binary: "perl", Category: domain.CategoryShellSpawn, Risk: domain.RiskHigh, Notes: "Perl (can spawn shell)", BuiltIn: true},
	"ruby":    {Binary: "ruby", Category: domain.CategoryShellSpawn, Risk: domain.RiskHigh, Notes: "Ruby (can spawn shell)", BuiltIn: true},
	"node":    {Binary: "node", Category: domain.CategoryShellSpawn, Risk: domain.RiskHigh, Notes: "Node.js", BuiltIn: true},
	"php":     {Binary: "php", Category: domain.CategoryShellSpawn, Risk: domain.RiskHigh, Notes: "PHP CLI", BuiltIn: true},
	"lua":     {Binary: "lua", Category: domain.CategoryShellSpawn, Risk: domain.RiskHigh, Notes: "Lua", BuiltIn: true},
	"tclsh":   {Binary: "tclsh", Category: domain.CategoryShellSpawn, Risk: domain.RiskHigh, Notes: "TCL shell", BuiltIn: true},
	"expect":  {Binary: "expect", Category: domain.CategoryShellSpawn, Risk: domain.RiskHigh, Notes: "Expect (interactive automation)", BuiltIn: true},
	"awk":     {Binary: "awk", Category: domain.CategoryShellSpawn, Risk: domain.RiskMedium, Notes: "Can execute system()", BuiltIn: true},
	"gawk":    {Binary: "gawk", Category: domain.CategoryShellSpawn, Risk: domain.RiskMedium, Notes: "GNU awk", BuiltIn: true},

	// --- Persistence ---
	"crontab":   {Binary: "crontab", Category: domain.CategoryPersistence, Risk: domain.RiskHigh, Notes: "Schedules cron jobs", BuiltIn: true},
	"at":        {Binary: "at", Category: domain.CategoryPersistence, Risk: domain.RiskHigh, Notes: "Schedules one-off jobs", BuiltIn: true},
	"batch":     {Binary: "batch", Category: domain.CategoryPersistence, Risk: domain.RiskHigh, Notes: "Schedules batch jobs", BuiltIn: true},
	"systemctl": {Binary: "systemctl", Category: domain.CategoryServiceControl, Risk: domain.RiskMedium, Notes: "systemd service control", BuiltIn: true},
	"service":   {Binary: "service", Category: domain.CategoryServiceControl, Risk: domain.RiskMedium, Notes: "SysV service control", BuiltIn: true},
	"launchctl": {Binary: "launchctl", Category: domain.CategoryServiceControl, Risk: domain.RiskMedium, Notes: "macOS service control", BuiltIn: true},

	// --- Package managers ---
	"apt":      {Binary: "apt", Category: domain.CategoryPackageManager, Risk: domain.RiskMedium, Notes: "Debian/Ubuntu package mgr", BuiltIn: true},
	"apt-get":  {Binary: "apt-get", Category: domain.CategoryPackageManager, Risk: domain.RiskMedium, Notes: "apt-get", BuiltIn: true},
	"aptitude": {Binary: "aptitude", Category: domain.CategoryPackageManager, Risk: domain.RiskMedium, Notes: "aptitude", BuiltIn: true},
	"dpkg":     {Binary: "dpkg", Category: domain.CategoryPackageManager, Risk: domain.RiskMedium, Notes: "Debian package tool", BuiltIn: true},
	"dnf":      {Binary: "dnf", Category: domain.CategoryPackageManager, Risk: domain.RiskMedium, Notes: "Fedora/RHEL package mgr", BuiltIn: true},
	"yum":      {Binary: "yum", Category: domain.CategoryPackageManager, Risk: domain.RiskMedium, Notes: "RHEL package mgr", BuiltIn: true},
	"rpm":      {Binary: "rpm", Category: domain.CategoryPackageManager, Risk: domain.RiskMedium, Notes: "RPM package manager", BuiltIn: true},
	"pacman":   {Binary: "pacman", Category: domain.CategoryPackageManager, Risk: domain.RiskMedium, Notes: "Arch package mgr", BuiltIn: true},
	"zypper":   {Binary: "zypper", Category: domain.CategoryPackageManager, Risk: domain.RiskMedium, Notes: "SUSE package mgr", BuiltIn: true},
	"brew":     {Binary: "brew", Category: domain.CategoryPackageManager, Risk: domain.RiskMedium, Notes: "Homebrew", BuiltIn: true},
	"snap":     {Binary: "snap", Category: domain.CategoryPackageManager, Risk: domain.RiskMedium, Notes: "Snap packages", BuiltIn: true},
	"flatpak":  {Binary: "flatpak", Category: domain.CategoryPackageManager, Risk: domain.RiskMedium, Notes: "Flatpak", BuiltIn: true},
	"pip":      {Binary: "pip", Category: domain.CategoryPackageManager, Risk: domain.RiskMedium, Notes: "Python pip", BuiltIn: true},
	"pip3":     {Binary: "pip3", Category: domain.CategoryPackageManager, Risk: domain.RiskMedium, Notes: "Python pip3", BuiltIn: true},
	"npm":      {Binary: "npm", Category: domain.CategoryPackageManager, Risk: domain.RiskMedium, Notes: "Node package mgr", BuiltIn: true},
	"yarn":     {Binary: "yarn", Category: domain.CategoryPackageManager, Risk: domain.RiskMedium, Notes: "Yarn", BuiltIn: true},
	"pnpm":     {Binary: "pnpm", Category: domain.CategoryPackageManager, Risk: domain.RiskMedium, Notes: "pnpm", BuiltIn: true},
	"gem":      {Binary: "gem", Category: domain.CategoryPackageManager, Risk: domain.RiskMedium, Notes: "RubyGems", BuiltIn: true},
	"cargo":    {Binary: "cargo", Category: domain.CategoryPackageManager, Risk: domain.RiskMedium, Notes: "Rust crates", BuiltIn: true},

	// --- Network config / firewalls ---
	"iptables":  {Binary: "iptables", Category: domain.CategoryNetworkConfig, Risk: domain.RiskHigh, Notes: "Linux firewall", BuiltIn: true},
	"ip6tables": {Binary: "ip6tables", Category: domain.CategoryNetworkConfig, Risk: domain.RiskHigh, Notes: "IPv6 firewall", BuiltIn: true},
	"nft":       {Binary: "nft", Category: domain.CategoryNetworkConfig, Risk: domain.RiskHigh, Notes: "nftables", BuiltIn: true},
	"ufw":       {Binary: "ufw", Category: domain.CategoryNetworkConfig, Risk: domain.RiskHigh, Notes: "Uncomplicated firewall", BuiltIn: true},
	"firewalld": {Binary: "firewalld", Category: domain.CategoryNetworkConfig, Risk: domain.RiskHigh, Notes: "firewalld", BuiltIn: true},
	"pfctl":     {Binary: "pfctl", Category: domain.CategoryNetworkConfig, Risk: domain.RiskHigh, Notes: "macOS PF firewall", BuiltIn: true},
	"ip":        {Binary: "ip", Category: domain.CategoryNetworkConfig, Risk: domain.RiskHigh, Notes: "iproute2", BuiltIn: true},
	"ifconfig":  {Binary: "ifconfig", Category: domain.CategoryNetworkConfig, Risk: domain.RiskHigh, Notes: "Legacy net config", BuiltIn: true},
	"route":     {Binary: "route", Category: domain.CategoryNetworkConfig, Risk: domain.RiskHigh, Notes: "Routing table", BuiltIn: true},

	// --- Filesystem privileged ops (when used on /etc or /root) ---
	"mount":  {Binary: "mount", Category: domain.CategoryFilesystem, Risk: domain.RiskHigh, Notes: "Mounts filesystems", BuiltIn: true},
	"umount": {Binary: "umount", Category: domain.CategoryFilesystem, Risk: domain.RiskHigh, Notes: "Unmounts filesystems", BuiltIn: true},
	"fdisk":  {Binary: "fdisk", Category: domain.CategoryFilesystem, Risk: domain.RiskHigh, Notes: "Partition table editor", BuiltIn: true},
	"mkfs":   {Binary: "mkfs", Category: domain.CategoryFilesystem, Risk: domain.RiskCritical, Notes: "Creates filesystems (DESTRUCTIVE)", BuiltIn: true},
	"dd":     {Binary: "dd", Category: domain.CategoryFilesystem, Risk: domain.RiskHigh, Notes: "Low-level copy (destructive)", BuiltIn: true},
}
