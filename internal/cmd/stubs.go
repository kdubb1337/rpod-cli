package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/kdubb1337/runpod-cli/internal/api"
	"github.com/kdubb1337/runpod-cli/internal/config"
	"github.com/kdubb1337/runpod-cli/internal/output"
)

// This file holds the Rung-3-floor commands: doctor, agent-context, profile, auth, skill-path.

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
const SchemaVersion = 1

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

// --- skill-path -------------------------------------------------------------

var skillPathCmd = &cobra.Command{
	Use:   "skill-path",
	Short: "Print the absolute path to the bundled SKILL.md",
	RunE: func(cmd *cobra.Command, args []string) error {
		exe, err := os.Executable()
		if err != nil {
			return err
		}
		dir := filepath.Dir(exe)
		candidates := []string{
			filepath.Join(dir, "..", "share", "rpod", "skills", "rpod", "SKILL.md"),
			filepath.Join(dir, "skills", "rpod", "SKILL.md"),
		}
		for _, p := range candidates {
			if _, err := os.Stat(p); err == nil {
				abs, _ := filepath.Abs(p)
				fmt.Fprintln(os.Stdout, abs)
				return nil
			}
		}
		return output.Errorf(1, "skill_not_found",
			"could not locate bundled SKILL.md; tried %v (os=%s)", candidates, runtime.GOOS)
	},
}

func init() {
	profileCmd.AddCommand(profileListCmd, profileSaveCmd, profileUseCmd, profileDeleteCmd)
	authCmd.AddCommand(authListCmd, authAddCmd, authRemoveCmd)
	rootCmd.AddCommand(doctorCmd, agentContextCmd, profileCmd, authCmd, skillPathCmd)
}
