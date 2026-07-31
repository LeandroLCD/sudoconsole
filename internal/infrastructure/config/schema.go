package config

import "github.com/LeandroLCD/sudoconsole/internal/domain"

// fileSchema is the on-disk TOML representation of the user config.
//
// It mirrors domain.Config field-for-field but uses TOML-friendly types
// (e.g. string enums, []string). The loader converts it to domain.Config
// after parsing so that domain types stay free of TOML concerns.
type fileSchema struct {
	Cache    cacheSection    `toml:"cache"`
	Security securitySection `toml:"security"`
	Policy   policySection   `toml:"policy"`
	Agent    agentSection    `toml:"agent"`
	Output   outputSection   `toml:"output"`
}

type cacheSection struct {
	TimeoutSeconds       int  `toml:"timeout_seconds"`
	RefreshBeforeSeconds int  `toml:"refresh_before_seconds"`
	Disabled             bool `toml:"disabled"`
}

type securitySection struct {
	PurgeMemory      bool `toml:"purge_memory"`
	DisableCoreDumps bool `toml:"disable_core_dumps"`
	RequireTTY       bool `toml:"require_tty"`
}

type policySection struct {
	Mode               string                 `toml:"mode"`
	Blocked            blocklistSection       `toml:"blocked"`
	Allowed            allowlistSection       `toml:"allowed"`
	Audit              auditSection           `toml:"audit"`
	ExtraPatterns      []string               `toml:"extra_patterns"`
	RemoteAccess       remoteAccessSection    `toml:"remote_access"`
	CredentialExposure credExposureSection    `toml:"credential_exposure"`
	Raw                map[string]interface{} `toml:"-"`
}

type blocklistSection struct {
	Categories []string `toml:"categories"`
	Commands   []string `toml:"commands"`
	Patterns   []string `toml:"patterns"`
}

type allowlistSection struct {
	Categories []string `toml:"categories"`
	Commands   []string `toml:"commands"`
	Patterns   []string `toml:"patterns"`
}

type auditSection struct {
	LogFile    string `toml:"log_file"`
	LogBlocked bool   `toml:"log_blocked"`
	LogAllowed bool   `toml:"log_allowed"`
	LogWarned  bool   `toml:"log_warned"`
	MaxBytes   int64  `toml:"max_bytes"`
}

type remoteAccessSection struct {
	BlockReverseTunnels bool `toml:"block_reverse_tunnels"`
	BlockPortForward    bool `toml:"block_port_forward"`
	BlockShellSpawn     bool `toml:"block_shell_spawn"`
	BlockTunnels        bool `toml:"block_tunnels"`
}

type credExposureSection struct {
	BlockPasswdChange bool `toml:"block_passwd_change"`
	BlockShadowEdit   bool `toml:"block_shadow_edit"`
	BlockSudoersEdit  bool `toml:"block_sudoers_edit"`
	BlockSSHKeyExport bool `toml:"block_ssh_key_export"`
	BlockSecretExport bool `toml:"block_secret_export"`
}

type agentSection struct {
	AutoDetect bool     `toml:"auto_detect"`
	Install    []string `toml:"install"`
	BinDir     string   `toml:"bin_dir"`
}

type outputSection struct {
	Format   string `toml:"format"`
	LogLevel string `toml:"log_level"`
}

// newFileSchemaFromConfig projects a domain.Config into the on-disk
// representation. It is the inverse of applyFileSchema; both helpers
// keep conversion logic in one place.
func newFileSchemaFromConfig(cfg domain.Config) fileSchema {
	cats := make([]string, 0, len(cfg.Policy.Blocked.Categories))
	for _, c := range cfg.Policy.Blocked.Categories {
		cats = append(cats, string(c))
	}
	allowedCats := make([]string, 0, len(cfg.Policy.Allowed.Categories))
	for _, c := range cfg.Policy.Allowed.Categories {
		allowedCats = append(allowedCats, string(c))
	}
	install := make([]string, 0, len(cfg.Agent.Install))
	for _, k := range cfg.Agent.Install {
		install = append(install, k.String())
	}
	return fileSchema{
		Cache: cacheSection{
			TimeoutSeconds:       cfg.Cache.TimeoutSeconds,
			RefreshBeforeSeconds: cfg.Cache.RefreshBeforeSeconds,
			Disabled:             cfg.Cache.Disabled,
		},
		Security: securitySection{
			PurgeMemory:      cfg.Security.PurgeMemory,
			DisableCoreDumps: cfg.Security.DisableCoreDumps,
			RequireTTY:       cfg.Security.RequireTTY,
		},
		Policy: policySection{
			Mode: cfg.Policy.Mode.String(),
			Blocked: blocklistSection{
				Categories: cats,
				Commands:   cfg.Policy.Blocked.Commands,
				Patterns:   cfg.Policy.Blocked.Patterns,
			},
			Allowed: allowlistSection{
				Categories: allowedCats,
				Commands:   cfg.Policy.Allowed.Commands,
				Patterns:   cfg.Policy.Allowed.Patterns,
			},
			Audit: auditSection{
				LogFile:    cfg.Policy.Audit.LogFile,
				LogBlocked: cfg.Policy.Audit.LogBlocked,
				LogAllowed: cfg.Policy.Audit.LogAllowed,
				LogWarned:  cfg.Policy.Audit.LogWarned,
				MaxBytes:   cfg.Policy.Audit.MaxBytes,
			},
			ExtraPatterns: cfg.Policy.ExtraPatterns,
			RemoteAccess: remoteAccessSection{
				BlockReverseTunnels: cfg.Policy.RemoteAccess.BlockReverseTunnels,
				BlockPortForward:    cfg.Policy.RemoteAccess.BlockPortForward,
				BlockShellSpawn:     cfg.Policy.RemoteAccess.BlockShellSpawn,
				BlockTunnels:        cfg.Policy.RemoteAccess.BlockTunnels,
			},
			CredentialExposure: credExposureSection{
				BlockPasswdChange: cfg.Policy.CredentialExposure.BlockPasswdChange,
				BlockShadowEdit:   cfg.Policy.CredentialExposure.BlockShadowEdit,
				BlockSudoersEdit:  cfg.Policy.CredentialExposure.BlockSudoersEdit,
				BlockSSHKeyExport: cfg.Policy.CredentialExposure.BlockSSHKeyExport,
				BlockSecretExport: cfg.Policy.CredentialExposure.BlockSecretExport,
			},
		},
		Agent: agentSection{
			AutoDetect: cfg.Agent.AutoDetect,
			Install:    install,
			BinDir:     cfg.Agent.BinDir,
		},
		Output: outputSection{
			Format:   cfg.Output.Format,
			LogLevel: cfg.Output.LogLevel,
		},
	}
}
