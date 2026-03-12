package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/malleus35/agentcom/internal/agent"
	"github.com/spf13/cobra"
)

const staleHeartbeatThreshold = 30 * time.Second

func newHealthCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "health",
		Short: "Check health of all registered agents",
		RunE: func(cmd *cobra.Command, args []string) error {
			registry := agent.NewRegistry(app.db, app.cfg)
			agents, err := registry.ListAll(cmd.Context())
			if err != nil {
				return fmt.Errorf("cli.newHealthCmd: list agents: %w", err)
			}

			type healthEntry struct {
				Name         string `json:"name"`
				Type         string `json:"type"`
				PID          int    `json:"pid"`
				HeartbeatAge string `json:"heartbeat_age"`
				Socket       bool   `json:"socket"`
				Verdict      string `json:"verdict"`
			}

			entries := make([]healthEntry, 0, len(agents))
			now := time.Now().UTC()

			for _, agt := range agents {
				pidAlive := processAliveCheck(agt.PID)
				socketOK := false
				if agt.SocketPath != "" {
					if _, err := os.Stat(agt.SocketPath); err == nil {
						socketOK = true
					}
				}

				hbTime, err := parseTimestamp(fmt.Sprint(agt.LastHeartbeat))
				if err != nil {
					return fmt.Errorf("cli.newHealthCmd: parse heartbeat: %w", err)
				}

				age := now.Sub(hbTime)
				verdict := "OK"
				switch {
				case !pidAlive:
					verdict = "DEAD"
				case age > staleHeartbeatThreshold:
					verdict = "STALE"
				}

				entries = append(entries, healthEntry{
					Name:         agt.Name,
					Type:         agt.Type,
					PID:          agt.PID,
					HeartbeatAge: age.Round(time.Second).String(),
					Socket:       socketOK,
					Verdict:      verdict,
				})
			}

			if jsonOutput {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(entries)
			}

			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			if _, err := fmt.Fprintln(tw, "NAME\tTYPE\tPID\tHEARTBEAT_AGE\tSOCKET\tVERDICT"); err != nil {
				return fmt.Errorf("cli.newHealthCmd: write header: %w", err)
			}
			for _, e := range entries {
				if _, err := fmt.Fprintf(tw, "%s\t%s\t%d\t%s\t%t\t%s\n", e.Name, e.Type, e.PID, e.HeartbeatAge, e.Socket, e.Verdict); err != nil {
					return fmt.Errorf("cli.newHealthCmd: write row: %w", err)
				}
			}
			if err := tw.Flush(); err != nil {
				return fmt.Errorf("cli.newHealthCmd: flush: %w", err)
			}

			return nil
		},
	}
}

func processAliveCheck(pid int) bool {
	if pid <= 0 {
		return false
	}

	err := syscall.Kill(pid, 0)
	if err == nil {
		return true
	}

	return errors.Is(err, syscall.EPERM)
}

func parseTimestamp(raw string) (time.Time, error) {
	if raw == "" {
		return time.Time{}, fmt.Errorf("cli.parseTimestamp: empty timestamp")
	}

	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05",
	}

	for _, layout := range layouts {
		t, err := time.Parse(layout, raw)
		if err == nil {
			return t.UTC(), nil
		}
	}

	return time.Time{}, fmt.Errorf("cli.parseTimestamp: unsupported format: %s", raw)
}
