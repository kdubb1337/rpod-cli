package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/kdubb1337/rpod-cli/internal/api"
	"github.com/kdubb1337/rpod-cli/internal/config"
	"github.com/kdubb1337/rpod-cli/internal/output"
)

// This file holds the Rung-3-floor commands: doctor, agent-context, profile, auth, skill.

// --- doctor -----------------------------------------------------------------

var doctorCmd = &cobra.Command{
	Use:     "doctor",
	Short:   "Health check: config, credentials, API reachability",
	Example: `  rpod doctor --json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		type check struct {
			Name   string `json:"name"`
			Status string `json:"status"`
			Detail string `json:"detail,omitempty"`
		}
		checks := []check{}

		// config file
		store, err := config.Load()
		if err != nil {
			checks = append(checks, check{Name: "config", Status: "fail", Detail: err.Error()})
		} else {
			checks = append(checks, check{
				Name:   "config",
				Status: "ok",
				Detail: fmt.Sprintf("%d profile(s) configured", len(store.Profiles)),
			})
		}

		// credentials
		cur := config.Current()
		if cur.APIKey == "" {
			checks = append(checks, check{
				Name:   "credentials",
				Status: "fail",
				Detail: "no API key (set RUNPOD_API_KEY or run `rpod auth add <key>`)",
			})
		} else {
			checks = append(checks, check{
				Name:   "credentials",
				Status: "ok",
				Detail: fmt.Sprintf("api key present (%d chars)", len(cur.APIKey)),
			})
		}

		// API reachability
		if cur.APIKey == "" {
			checks = append(checks, check{Name: "api_reachable", Status: "skipped", Detail: "no credentials"})
		} else {
			ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
			defer cancel()
			if _, err := api.New(cur.APIKey).ListGPUTypes(ctx); err != nil {
				checks = append(checks, check{Name: "api_reachable", Status: "fail", Detail: err.Error()})
			} else {
				checks = append(checks, check{Name: "api_reachable", Status: "ok"})
			}
		}

		ok := true
		for _, c := range checks {
			if c.Status == "fail" {
				ok = false
				break
			}
		}
		report := struct {
			OK     bool    `json:"ok"`
			Checks []check `json:"checks"`
		}{OK: ok, Checks: checks}

		if err := output.Emit(report); err != nil {
			return err
		}
		if !ok {
			return output.Errorf(1, "doctor_failed", "one or more health checks failed")
		}
		return nil
	},
}

// --- agent-context ----------------------------------------------------------

// SchemaVersion is bumped on any breaking change to the agent-context output shape.
//
//	v1 — initial pod/volume/gpu surface.
//	v2 — pod ssh-info, url, wait, exec, cp; pod create --gpu-type repeatable,
//	     --ssh-key-file, --wait; capacity-error envelope (code=capacity).
const SchemaVersion = 2

var agentContextCmd = &cobra.Command{
	Use:   "agent-context",
	Short: "Emit versioned structured introspection for AI agents",
	Long: `Emits a JSON document describing all commands, flags, enums, profiles,
and exit codes. Agents read this once instead of crawling --help.

Bumps schema_version on breaking shape changes.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := buildAgentContext(rootCmd)
		out, err := json.MarshalIndent(ctx, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(os.Stdout, string(out))
		return nil
	},
}

func buildAgentContext(root *cobra.Command) map[string]any {
	return map[string]any{
		"schema_version": SchemaVersion,
		"cli":            root.Name(),
		"version":        version,
		"exit_codes": map[string]int{
			"ok": 0, "generic": 1, "usage": 2, "not_found": 3, "auth": 4,
			"api": 5, "conflict": 6, "rate_limit": 7, "network": 8,
			"validation": 9, "timeout": 124,
		},
		"error_codes": map[string]string{
			// Non-exhaustive — these are the kinds rpod classifies in the
			// JSON error envelope's `code` field. Branch on these instead of
			// substring-matching the message.
			"capacity":              "GPU/region is out of capacity right now; retry with a different --gpu-type or --data-center, or wait",
			"timeout":               "operation did not complete within the deadline (pod wait, etc.)",
			"auth_missing":          "no API key configured",
			"confirmation_required": "destructive op needs --force or --yes",
		},
		"enums": map[string]any{
			"pod_create.cloud_type": validCloudTypes,
		},
		"commands": describeCommands(root),
	}
}

func describeCommands(c *cobra.Command) []map[string]any {
	out := make([]map[string]any, 0, len(c.Commands()))
	for _, sub := range c.Commands() {
		if sub.Hidden || !sub.IsAvailableCommand() {
			continue
		}
		flags := []map[string]any{}
		sub.LocalFlags().VisitAll(func(f *pflag.Flag) {
			flags = append(flags, map[string]any{
				"name":    f.Name,
				"type":    f.Value.Type(),
				"default": f.DefValue,
				"usage":   f.Usage,
			})
		})
		entry := map[string]any{
			"name":     sub.Name(),
			"use":      sub.Use,
			"short":    sub.Short,
			"example":  sub.Example,
			"flags":    flags,
			"children": describeCommands(sub),
		}
		out = append(out, entry)
	}
	return out
}

// --- profile ----------------------------------------------------------------

var profileCmd = &cobra.Command{
	Use:   "profile",
	Short: "Manage named configuration profiles",
}

var profileListCmd = &cobra.Command{
	Use:   "list",
	Short: "List saved profiles",
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := config.Load()
		if err != nil {
			return err
		}
		names := make([]string, 0, len(store.Profiles))
		for n := range store.Profiles {
			names = append(names, n)
		}
		sort.Strings(names)
		return output.Emit(map[string]any{
			"profiles":        names,
			"default_profile": store.DefaultProfile,
		})
	},
}

var profileSaveCmd = &cobra.Command{
	Use:   "save <name>",
	Short: "Save the current resolved configuration as a named profile",
	Args:  cobra.ExactArgs(1),
	Example: `  RUNPOD_API_KEY=... rpod profile save default
  rpod profile save staging --account org_xyz`,
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := config.Load()
		if err != nil {
			return err
		}
		cur := config.Current()
		if cur.APIKey == "" {
			return output.ErrorfHint(2, "no_api_key",
				"set RUNPOD_API_KEY or run `rpod auth add <key>` first",
				"cannot save profile without an API key")
		}
		store.Profiles[args[0]] = config.Profile{
			Account: cur.Account,
			APIKey:  cur.APIKey,
		}
		if store.DefaultProfile == "" {
			store.DefaultProfile = args[0]
		}
		if err := config.Save(store); err != nil {
			return err
		}
		return output.Emit(map[string]any{"saved": args[0]})
	},
}

var profileUseCmd = &cobra.Command{
	Use:     "use <name>",
	Short:   "Set the default profile",
	Args:    cobra.ExactArgs(1),
	Example: `  rpod profile use staging`,
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := config.Load()
		if err != nil {
			return err
		}
		if _, ok := store.Profiles[args[0]]; !ok {
			names := make([]string, 0, len(store.Profiles))
			for n := range store.Profiles {
				names = append(names, n)
			}
			sort.Strings(names)
			return output.ErrorfEnum(3, "profile_not_found", names,
				"profile %q not found", args[0])
		}
		store.DefaultProfile = args[0]
		if err := config.Save(store); err != nil {
			return err
		}
		return output.Emit(map[string]any{"default_profile": args[0]})
	},
}

var profileDeleteCmd = &cobra.Command{
	Use:     "delete <name>",
	Short:   "Delete a saved profile",
	Args:    cobra.ExactArgs(1),
	Example: `  rpod profile delete staging --force`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if !flagForce && !flagYes {
			return output.ErrorfHint(2, "confirmation_required",
				"pass --force or --yes to confirm",
				"refusing to delete profile %s without confirmation", args[0])
		}
		store, err := config.Load()
		if err != nil {
			return err
		}
		if _, ok := store.Profiles[args[0]]; !ok {
			return output.Errorf(3, "profile_not_found", "profile %q not found", args[0])
		}
		delete(store.Profiles, args[0])
		if store.DefaultProfile == args[0] {
			store.DefaultProfile = ""
		}
		if err := config.Save(store); err != nil {
			return err
		}
		return output.Emit(map[string]any{"deleted": args[0]})
	},
}

// --- auth -------------------------------------------------------------------

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Manage RunPod API credentials",
}

var authListCmd = &cobra.Command{
	Use:   "list",
	Short: "List configured profiles (credentials live inside profiles)",
	RunE: func(cmd *cobra.Command, args []string) error {
		return profileListCmd.RunE(cmd, args)
	},
}

var authAddCmd = &cobra.Command{
	Use:   "add <api-key>",
	Short: "Save an API key as a named profile",
	Args:  cobra.ExactArgs(1),
	Long: `Stores the RunPod API key in ~/.rpod/config.json (mode 0600).

For a TTY-friendly flow you can also export RUNPOD_API_KEY and skip this command entirely.`,
	Example: `  rpod auth add rpa_xxx --profile default
  rpod auth add rpa_yyy --profile staging`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Reuse the persistent --profile flag (defined on root).
		target := flagProfile
		if target == "" {
			target = "default"
		}
		store, err := config.Load()
		if err != nil {
			return err
		}
		existing := store.Profiles[target]
		existing.APIKey = args[0]
		store.Profiles[target] = existing
		if store.DefaultProfile == "" {
			store.DefaultProfile = target
		}
		if err := config.Save(store); err != nil {
			return err
		}
		return output.Emit(map[string]any{"saved_profile": target})
	},
}

var authRemoveCmd = &cobra.Command{
	Use:   "remove <profile>",
	Short: "Remove an API key from a profile",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if !flagForce && !flagYes {
			return output.ErrorfHint(2, "confirmation_required",
				"pass --force or --yes to confirm",
				"refusing to clear credentials for profile %s without confirmation", args[0])
		}
		store, err := config.Load()
		if err != nil {
			return err
		}
		p, ok := store.Profiles[args[0]]
		if !ok {
			return output.Errorf(3, "profile_not_found", "profile %q not found", args[0])
		}
		p.APIKey = ""
		store.Profiles[args[0]] = p
		if err := config.Save(store); err != nil {
			return err
		}
		return output.Emit(map[string]any{"cleared": args[0]})
	},
}

// --- skill ------------------------------------------------------------------
//
// Manage the SKILL.md that ships inside this binary. Agents discover the CLI by
// dropping the bundled skill folder into their personal skills directory:
//
//   rpod skill install claude codex   # symlink into ~/.claude/skills and ~/.codex/skills
//   rpod skill install --all          # every agent whose parent dir exists
//   rpod skill list                   # show install status across known agents
//   rpod skill path                   # print the source SKILL.md path

type agentTarget struct {
	Name  string
	Dir   string
	Notes string
}

func agentRegistry() []agentTarget {
	home, _ := os.UserHomeDir()
	expand := func(envKey, fallback string) string {
		if v := os.Getenv(envKey); v != "" {
			return v
		}
		return filepath.Join(home, fallback)
	}
	return []agentTarget{
		{Name: "claude", Dir: expand("RPOD_SKILLS_CLAUDE", ".claude/skills"), Notes: "Claude Code (Anthropic)"},
		{Name: "codex", Dir: expand("RPOD_SKILLS_CODEX", ".codex/skills"), Notes: "Codex CLI (OpenAI)"},
		{Name: "gemini", Dir: expand("RPOD_SKILLS_GEMINI", ".gemini/skills"), Notes: "Gemini CLI (Google)"},
		{Name: "openhands", Dir: expand("RPOD_SKILLS_OPENHANDS", ".openhands/microagents"), Notes: "OpenHands (V0 microagents path)"},
		{Name: "agents", Dir: expand("RPOD_SKILLS_AGENTS", ".agents/skills"), Notes: "Cross-agent universal (Gemini, OpenHands V1)"},
	}
}

func agentNames() []string {
	reg := agentRegistry()
	names := make([]string, 0, len(reg))
	for _, a := range reg {
		names = append(names, a.Name)
	}
	return names
}

func lookupAgent(name string) (agentTarget, bool) {
	for _, a := range agentRegistry() {
		if a.Name == name {
			return a, true
		}
	}
	return agentTarget{}, false
}

func findSkillSource() (dir string, file string, err error) {
	exe, err := os.Executable()
	if err != nil {
		return "", "", err
	}
	base := filepath.Dir(exe)
	candidates := []string{
		filepath.Join(base, "..", "share", "rpod", "skills", "rpod"),
		filepath.Join(base, "skills", "rpod"),
		filepath.Join(base, "..", "skills", "rpod"),
	}
	for _, c := range candidates {
		p := filepath.Join(c, "SKILL.md")
		if _, statErr := os.Stat(p); statErr == nil {
			abs, _ := filepath.Abs(c)
			return abs, filepath.Join(abs, "SKILL.md"), nil
		}
	}
	return "", "", output.Errorf(1, "skill_not_found",
		"could not locate bundled SKILL.md; tried %v (os=%s)", candidates, runtime.GOOS)
}

var skillCmd = &cobra.Command{
	Use:   "skill",
	Short: "Manage the bundled SKILL.md (path / install / uninstall / list)",
	Long: `Manage the SKILL.md that ships inside this binary so agents can discover
rpod's verbs and conventions.

Known agent targets:
  claude     ~/.claude/skills        Claude Code (Anthropic)
  codex      ~/.codex/skills         Codex CLI (OpenAI)
  gemini     ~/.gemini/skills        Gemini CLI (Google)
  openhands  ~/.openhands/microagents OpenHands (V0)
  agents     ~/.agents/skills        Cross-agent universal path

Override any target's path with $RPOD_SKILLS_<AGENT> (e.g.
RPOD_SKILLS_CLAUDE=/opt/skills/claude).`,
}

var skillPathCmd = &cobra.Command{
	Use:   "path",
	Short: "Print the absolute path to the bundled SKILL.md",
	RunE: func(cmd *cobra.Command, args []string) error {
		_, file, err := findSkillSource()
		if err != nil {
			return err
		}
		fmt.Fprintln(os.Stdout, file)
		return nil
	},
}

var (
	flagSkillInstallMode string
	flagSkillInstallAll  bool
	flagSkillInstallDir  string
)

var skillInstallCmd = &cobra.Command{
	Use:   "install [agent...]",
	Short: "Install the bundled SKILL.md into one or more agent skills directories",
	Example: `  rpod skill install claude
  rpod skill install claude codex gemini
  rpod skill install --all
  rpod skill install --dir ~/.config/myagent/skills
  rpod skill install claude --mode=copy --force --dry-run`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runSkillInstall(args, false)
	},
}

var skillUninstallCmd = &cobra.Command{
	Use:   "uninstall [agent...]",
	Short: "Remove the bundled SKILL.md from one or more agent skills directories",
	Example: `  rpod skill uninstall claude
  rpod skill uninstall --all`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runSkillInstall(args, true)
	},
}

func runSkillInstall(args []string, remove bool) error {
	if flagSkillInstallMode != "symlink" && flagSkillInstallMode != "copy" {
		return output.ErrorfEnum(2, "bad_mode",
			[]string{"symlink", "copy"},
			"--mode must be 'symlink' or 'copy' (got %q)", flagSkillInstallMode)
	}

	srcDir, _, err := findSkillSource()
	if err != nil {
		return err
	}

	targets, err := resolveTargets(args)
	if err != nil {
		return err
	}

	type result struct {
		Agent  string `json:"agent"`
		Path   string `json:"path"`
		Mode   string `json:"mode,omitempty"`
		Action string `json:"action"`
		Detail string `json:"detail,omitempty"`
	}
	results := make([]result, 0, len(targets))

	for _, t := range targets {
		dest := filepath.Join(t.Dir, "rpod")
		r := result{Agent: t.Name, Path: dest, Mode: flagSkillInstallMode}

		if remove {
			r.Action, r.Detail, err = uninstallOne(dest)
		} else {
			r.Action, r.Detail, err = installOne(srcDir, dest, flagSkillInstallMode, flagForce, flagDryRun)
		}
		if err != nil {
			r.Action = "error"
			r.Detail = err.Error()
		}
		results = append(results, r)
	}

	verb := "install"
	if remove {
		verb = "uninstall"
	}
	payload := map[string]any{
		"action":  verb,
		"mode":    flagSkillInstallMode,
		"source":  srcDir,
		"dry_run": flagDryRun,
		"results": results,
	}
	if flagDryRun {
		return output.EmitDryRun(payload)
	}
	return output.Emit(payload)
}

func resolveTargets(args []string) ([]agentTarget, error) {
	reg := agentRegistry()
	picked := make([]agentTarget, 0, len(reg)+len(args)+1)

	if flagSkillInstallAll {
		picked = append(picked, reg...)
	}

	for _, name := range args {
		a, ok := lookupAgent(name)
		if !ok {
			return nil, output.ErrorfEnum(2, "unknown_agent",
				agentNames(),
				"unknown agent %q", name)
		}
		picked = append(picked, a)
	}

	if flagSkillInstallDir != "" {
		expanded := flagSkillInstallDir
		if strings.HasPrefix(expanded, "~/") {
			home, _ := os.UserHomeDir()
			expanded = filepath.Join(home, expanded[2:])
		}
		abs, _ := filepath.Abs(expanded)
		picked = append(picked, agentTarget{Name: "custom", Dir: abs, Notes: "user-specified --dir"})
	}

	if len(picked) == 0 {
		return nil, output.ErrorfEnum(2, "no_target",
			append(agentNames(), "--all", "--dir"),
			"no install target given; pass one or more of %v, or --all, or --dir <path>", agentNames())
	}

	seen := map[string]bool{}
	deduped := make([]agentTarget, 0, len(picked))
	for _, t := range picked {
		if seen[t.Dir] {
			continue
		}
		seen[t.Dir] = true
		deduped = append(deduped, t)
	}
	sort.SliceStable(deduped, func(i, j int) bool { return deduped[i].Name < deduped[j].Name })
	return deduped, nil
}

func installOne(srcDir, dest, mode string, force, dryRun bool) (string, string, error) {
	parent := filepath.Dir(dest)

	existing, statErr := os.Lstat(dest)
	if statErr != nil && !os.IsNotExist(statErr) {
		return "error", "", statErr
	}

	if existing != nil && existing.Mode()&os.ModeSymlink != 0 {
		target, _ := os.Readlink(dest)
		resolved, _ := filepath.Abs(target)
		if resolved == srcDir && mode == "symlink" {
			return "skipped", "symlink already points at source", nil
		}
	}

	if existing != nil && !force {
		return "skipped", fmt.Sprintf("destination exists; pass --force to overwrite (%s)", dest), nil
	}

	action, intent := "created", "create"
	if existing != nil {
		action, intent = "refreshed", "refresh"
	}
	if dryRun {
		return "would-" + intent, fmt.Sprintf("%s → %s (mode=%s)", srcDir, dest, mode), nil
	}

	if err := os.MkdirAll(parent, 0o755); err != nil {
		return "error", "", err
	}
	if existing != nil {
		if err := os.RemoveAll(dest); err != nil {
			return "error", "", err
		}
	}

	switch mode {
	case "symlink":
		if err := os.Symlink(srcDir, dest); err != nil {
			return "error", "", err
		}
	case "copy":
		if err := copyDir(srcDir, dest); err != nil {
			return "error", "", err
		}
	}
	return action, fmt.Sprintf("%s → %s", srcDir, dest), nil
}

func uninstallOne(dest string) (string, string, error) {
	info, err := os.Lstat(dest)
	if err != nil {
		if os.IsNotExist(err) {
			return "skipped", "not installed", nil
		}
		return "error", "", err
	}
	if flagDryRun {
		kind := "directory"
		if info.Mode()&os.ModeSymlink != 0 {
			kind = "symlink"
		}
		return "would-remove", fmt.Sprintf("remove %s at %s", kind, dest), nil
	}
	if err := os.RemoveAll(dest); err != nil {
		return "error", "", err
	}
	return "removed", dest, nil
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode().Perm())
		}
		return copyFile(path, target, info.Mode().Perm())
	})
}

func copyFile(src, dst string, perm os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

var skillListCmd = &cobra.Command{
	Use:   "list",
	Short: "Show the bundled SKILL.md install status across known agents",
	RunE: func(cmd *cobra.Command, args []string) error {
		srcDir, _, err := findSkillSource()
		if err != nil {
			return err
		}
		type row struct {
			Agent     string `json:"agent"`
			Path      string `json:"path"`
			Notes     string `json:"notes"`
			Installed bool   `json:"installed"`
			Mode      string `json:"mode,omitempty"`
			LinksToUs bool   `json:"links_to_us,omitempty"`
		}
		var rows []row
		for _, t := range agentRegistry() {
			dest := filepath.Join(t.Dir, "rpod")
			r := row{Agent: t.Name, Path: dest, Notes: t.Notes}
			if info, err := os.Lstat(dest); err == nil {
				r.Installed = true
				if info.Mode()&os.ModeSymlink != 0 {
					r.Mode = "symlink"
					if target, err := os.Readlink(dest); err == nil {
						resolved, _ := filepath.Abs(target)
						r.LinksToUs = resolved == srcDir
					}
				} else if info.IsDir() {
					r.Mode = "copy"
				} else {
					r.Mode = "file"
				}
			}
			rows = append(rows, r)
		}
		return output.Emit(map[string]any{
			"source":  srcDir,
			"targets": rows,
		})
	},
}

func init() {
	profileCmd.AddCommand(profileListCmd, profileSaveCmd, profileUseCmd, profileDeleteCmd)
	authCmd.AddCommand(authListCmd, authAddCmd, authRemoveCmd)

	skillInstallCmd.Flags().StringVar(&flagSkillInstallMode, "mode", "symlink", "install mode: symlink|copy")
	skillInstallCmd.Flags().BoolVar(&flagSkillInstallAll, "all", false, "install to every known agent in the registry")
	skillInstallCmd.Flags().StringVar(&flagSkillInstallDir, "dir", "", "additional custom skills directory to install into")
	skillUninstallCmd.Flags().BoolVar(&flagSkillInstallAll, "all", false, "uninstall from every known agent in the registry")
	skillUninstallCmd.Flags().StringVar(&flagSkillInstallDir, "dir", "", "additional custom skills directory to uninstall from")

	skillCmd.AddCommand(skillPathCmd, skillInstallCmd, skillUninstallCmd, skillListCmd)
	rootCmd.AddCommand(doctorCmd, agentContextCmd, profileCmd, authCmd, skillCmd)
}
