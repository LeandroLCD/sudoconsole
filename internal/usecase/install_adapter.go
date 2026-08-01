package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/LeandroLCD/sudoconsole/internal/domain"
)

// ErrInstallAborted is returned by InstallAdapterUseCase when the user
// rejects the interactive confirmation prompt.
var ErrInstallAborted = errors.New("install: aborted by user")

// InstallAdapterInput is the input to InstallAdapterUseCase.
type InstallAdapterInput struct {
	// Config is the effective configuration (file + flag overrides).
	// The use case does NOT mutate it; when PolicyModeOverride is set
	// a defensive copy is made.
	Config domain.Config

	// Kinds restricts the operation to the listed agent kinds. When
	// empty, the use case auto-detects installed agents via the
	// configured AgentDetector and uses every detected kind whose
	// AgentDescriptor.Available == true.
	Kinds []domain.AgentKind

	// Force allows overwriting existing integration files.
	Force bool

	// DryRun reports what would change without touching the host
	// filesystem. When true the Confirm callback is skipped.
	DryRun bool

	// BinDir overrides domain.Config.Agent.BinDir. Empty means
	// "use the value from Config".
	BinDir string

	// PolicyModeOverride, when non-zero, replaces Config.Policy.Mode
	// for this invocation only. It does not persist.
	PolicyModeOverride domain.PolicyMode

	// Uninstall reverses Install: it removes every adapter block
	// written by a previous install.
	Uninstall bool

	// Yes skips the interactive confirmation. The CLI passes true
	// when --yes is set or when stdin is not a TTY.
	Yes bool

	// Confirm, when non-nil, is invoked with a human-readable
	// summary of the planned changes. It must return true to
	// proceed, false to abort (returning ErrInstallAborted).
	//
	// Confirm is only called when:
	//   - !Yes && !DryRun && !Uninstall
	//   - there is at least one plan entry
	//
	// The CLI typically passes nil on non-TTY stdin (auto-accept)
	// or when the user passes --yes.
	Confirm func(message string) (bool, error)

	// Time is injectable for tests; defaults to time.Now().UTC().
	Time time.Time
}

// InstallOutcome describes a single successful (or planned) operation.
type InstallOutcome struct {
	Kind   domain.AgentKind `json:"kind"`
	Name   string           `json:"name"`
	Target string           `json:"target,omitempty"`
	DryRun bool             `json:"dry_run,omitempty"`
}

// InstallFailure describes a single failed operation.
type InstallFailure struct {
	Kind  domain.AgentKind `json:"kind"`
	Error string           `json:"error"`
}

// InstallAdapterOutput is the aggregate result returned to the
// transport layer.
type InstallAdapterOutput struct {
	Installed []InstallOutcome `json:"installed"`
	Skipped   []string         `json:"skipped,omitempty"`
	Failed    []InstallFailure `json:"failed,omitempty"`
	Time      string           `json:"time,omitempty"`

	// Plan is true when the use case produced a preview (--dry-run)
	// or was aborted by Confirm.
	Plan bool `json:"-"`
}

// InstallerResolver returns an AgentInstaller for the supplied kind.
// It is an indirection so the use case does not depend on the agent
// infrastructure package.
type InstallerResolver func(kind domain.AgentKind) (domain.AgentInstaller, error)

// InstallAdapterUseCase orchestrates the install/uninstall workflow:
//
//  1. Resolve target kinds (explicit list or auto-detect).
//  2. Apply the PolicyMode override (if any) and build InstallOptions.
//  3. Optionally ask the user to confirm the plan.
//  4. Invoke Install / Uninstall on every resolved adapter.
//
// The use case depends only on domain ports and is therefore trivially
// testable with fake detectors + fake installers.
type InstallAdapterUseCase struct {
	Detector     domain.AgentDetector
	ResolveAgent InstallerResolver
	Audit        domain.AuditLogger
	Now          func() string
}

// NewInstallAdapterUseCase constructs the use case. Either Detector or
// ResolveAgent may be nil; nil Detector means auto-detection is
// disabled (callers MUST supply explicit Kinds). nil Audit is
// tolerated and treated as a noop logger.
func NewInstallAdapterUseCase(
	det domain.AgentDetector,
	resolve InstallerResolver,
	audit domain.AuditLogger,
) *InstallAdapterUseCase {
	if audit == nil {
		audit = nilAudit{}
	}
	return &InstallAdapterUseCase{
		Detector:     det,
		ResolveAgent: resolve,
		Audit:        audit,
		Now:          defaultNowRFC3339,
	}
}

// Execute runs the workflow. It NEVER returns an error for per-adapter
// failures: those are collected into Output.Failed. The only errors
// returned are:
//
//   - context.Canceled / context.DeadlineExceeded
//   - ErrInstallAborted (user rejected confirmation)
//   - ErrInvalidInput (empty kinds + no detector, or unknown kind)
//   - errors from the resolver that affect every kind (e.g. agent
//     package not linked)
func (u *InstallAdapterUseCase) Execute(ctx context.Context, in InstallAdapterInput) (InstallAdapterOutput, error) {
	if u.ResolveAgent == nil {
		return InstallAdapterOutput{}, errors.New("install: resolver not configured")
	}

	kinds, err := u.resolveKinds(ctx, in)
	if err != nil {
		return InstallAdapterOutput{}, err
	}
	if len(kinds) == 0 {
		return InstallAdapterOutput{Time: u.Now(), Plan: in.DryRun}, nil
	}

	opts := u.buildOptions(in)

	// Confirmation gate (skipped for dry-run and uninstall).
	if !in.Yes && !in.DryRun && !in.Uninstall && in.Confirm != nil {
		msg := renderPlan(in.Uninstall, in.DryRun, kinds, opts)
		ok, cerr := in.Confirm(msg)
		if cerr != nil {
			return InstallAdapterOutput{}, fmt.Errorf("install: confirm: %w", cerr)
		}
		if !ok {
			return InstallAdapterOutput{Time: u.Now(), Plan: true}, ErrInstallAborted
		}
	}

	out := InstallAdapterOutput{Time: u.Now(), Plan: in.DryRun}
	for _, kind := range kinds {
		inst, err := u.ResolveAgent(kind)
		if err != nil {
			out.Failed = append(out.Failed, InstallFailure{Kind: kind, Error: err.Error()})
			continue
		}
		if in.Uninstall {
			if err := inst.Uninstall(ctx); err != nil {
				out.Failed = append(out.Failed, InstallFailure{Kind: kind, Error: err.Error()})
				continue
			}
			out.Installed = append(out.Installed, InstallOutcome{
				Kind:   kind,
				Name:   inst.Name().DisplayName(),
				DryRun: in.DryRun,
			})
			continue
		}
		if err := inst.Install(ctx, opts); err != nil {
			// Idempotent no-op when the marker is already present is
			// surfaced as success, not failure — adapters signal that
			// by returning nil from guardedInstall. Any non-nil error
			// is a real failure.
			out.Failed = append(out.Failed, InstallFailure{Kind: kind, Error: err.Error()})
			continue
		}
		out.Installed = append(out.Installed, InstallOutcome{
			Kind:   kind,
			Name:   inst.Name().DisplayName(),
			DryRun: in.DryRun,
		})
	}

	return out, nil
}

// resolveKinds returns the list of agent kinds to operate on.
//
//   - If in.Kinds is non-empty, it is returned verbatim (after filtering
//     out AgentUnknown and AgentGeneric unless explicitly requested).
//   - Otherwise the detector is consulted and every Available descriptor
//     contributes its kind.
func (u *InstallAdapterUseCase) resolveKinds(ctx context.Context, in InstallAdapterInput) ([]domain.AgentKind, error) {
	if len(in.Kinds) > 0 {
		out := make([]domain.AgentKind, 0, len(in.Kinds))
		for _, k := range in.Kinds {
			if k == domain.AgentUnknown {
				continue
			}
			out = append(out, k)
		}
		return out, nil
	}
	if u.Detector == nil {
		return nil, fmt.Errorf("%w: --kind is required when auto-detect is disabled",
			domain.ErrInvalidInput)
	}
	desc, err := u.Detector.Detect(ctx)
	if err != nil {
		return nil, fmt.Errorf("install: detect: %w", err)
	}
	out := make([]domain.AgentKind, 0, len(desc))
	for _, d := range desc {
		if !d.Available {
			continue
		}
		if d.Kind == domain.AgentUnknown || d.Kind == domain.AgentGeneric {
			continue
		}
		out = append(out, d.Kind)
	}
	return out, nil
}

// buildOptions assembles the domain.InstallOptions from the input and
// the effective configuration (with the PolicyMode override applied).
func (u *InstallAdapterUseCase) buildOptions(in InstallAdapterInput) domain.InstallOptions {
	cfg := in.Config
	if in.PolicyModeOverride != domain.PolicyModeUnknown {
		cfg.Policy.Mode = in.PolicyModeOverride
	}
	binDir := in.BinDir
	if binDir == "" {
		binDir = cfg.Agent.BinDir
	}
	return domain.InstallOptions{
		Force:  in.Force,
		DryRun: in.DryRun,
		BinDir: binDir,
		Config: cfg,
	}
}

// renderPlan formats the planned changes for the confirmation prompt.
func renderPlan(uninstall, dryRun bool, kinds []domain.AgentKind, opts domain.InstallOptions) string {
	verb := "Install"
	if uninstall {
		verb = "Uninstall"
	}
	if dryRun {
		verb = "Would " + strings.ToLower(verb)
	}
	var sb strings.Builder
	_, _ = fmt.Fprintf(&sb, "%s the following adapter(s):\n", verb)
	for _, k := range kinds {
		_, _ = fmt.Fprintf(&sb, "  - %s (%s)\n", k.DisplayName(), k.String())
	}
	if !uninstall {
		if opts.Force {
			_, _ = sb.WriteString("  force: overwrite existing integrations\n")
		}
		if opts.BinDir != "" {
			_, _ = fmt.Fprintf(&sb, "  bin_dir: %s\n", opts.BinDir)
		}
		if opts.Config.Policy.Mode != domain.PolicyModeUnknown {
			_, _ = fmt.Fprintf(&sb, "  policy_mode: %s\n", opts.Config.Policy.Mode)
		}
	}
	_, _ = sb.WriteString("Proceed? [y/N]: ")
	return sb.String()
}

// defaultNowRFC3339 is the default timestamp provider for the
// install output's Time field. It is overridable via UseCase.Now.
func defaultNowRFC3339() string {
	return time.Now().UTC().Format(time.RFC3339Nano)
}
