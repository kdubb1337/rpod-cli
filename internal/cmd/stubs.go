package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/kdubb1337/runpod-cli/internal/output"
)

// This file holds minimum-viable stubs for the four "Rung 3 floor" commands:
//   - doctor          : health check across config + creds + API
//   - agent-context   : versioned structured introspection
//   - profile         : save / use / list / show / delete
//   - auth            : add / list / remove
//
// Each is wired into the root command and produces the right output shape.
// Replace the bodies with real implementations as you build.

// --- doctor -----------------------------------------------------------------

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Health check: config, credentials, API reachability",
	Example: `  runpod doctor --json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		type check struct {
			Name   string `json:"name"`
			Status string `json:"status"`
			Detail string `json:"detail,omitempty"`
		}
		report := struct {
			OK     bool    `json:"ok"`
			Checks []check `json:"checks"`
		}{
			OK: true,
			Checks: []check{
				{Name: "config", Status: "ok"},
				{Name: "credentials", Status: "skipped", Detail: "no auth backend wired yet"},
				{Name: "api_reachable", Status: "skipped", Detail: "no API client wired yet"},
			},
		}
		return output.Emit(report)
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
		"commands": describeCommands(root),
	}
}

func describeCommands(c *cobra.Command) []map[string]any {
	var out []map[string]any
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
		return output.Emit(map[string]any{
			"profiles": []string{},
			"hint":     "no profiles saved; create one with: runpod profile save <name>",
		})
	},
}

// Stubs for save/use/show/delete go here; same pattern.

// --- auth -------------------------------------------------------------------

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Manage credentials and accounts",
}

var authListCmd = &cobra.Command{
	Use:   "list",
	Short: "List configured accounts",
	RunE: func(cmd *cobra.Command, args []string) error {
		return output.Emit(map[string]any{
			"accounts": []string{},
			"hint":     "no accounts configured; add one with: runpod auth add <id>",
		})
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
		// Resolve common install locations: Homebrew puts share/ alongside bin/.
		dir := filepath.Dir(exe)
		candidates := []string{
			filepath.Join(dir, "..", "share", "runpod", "skills", "runpod", "SKILL.md"),
			filepath.Join(dir, "skills", "runpod", "SKILL.md"),
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
	profileCmd.AddCommand(profileListCmd)
	authCmd.AddCommand(authListCmd)
	rootCmd.AddCommand(doctorCmd, agentContextCmd, profileCmd, authCmd, skillPathCmd)
}
