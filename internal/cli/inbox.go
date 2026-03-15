package cli

import (
	"encoding/json"
	"fmt"
	"text/tabwriter"

	"github.com/malleus35/agentcom/internal/agent"
	"github.com/malleus35/agentcom/internal/db"
	"github.com/malleus35/agentcom/internal/message"
	"github.com/spf13/cobra"
)

func newInboxCmd() *cobra.Command {
	var agentName string
	var unreadOnly bool
	var fromFilter string

	cmd := &cobra.Command{
		Use:   "inbox",
		Short: "View messages for an agent inbox",
		RunE: func(cmd *cobra.Command, args []string) error {
			registry := agent.NewRegistry(app.db, app.cfg)
			agt, err := registry.FindByName(cmd.Context(), agentName, currentProjectFilter())
			if err != nil {
				agt, err = registry.FindByID(cmd.Context(), agentName)
				if err != nil {
					return fmt.Errorf("cli.newInboxCmd: resolve agent: %w", err)
				}
			}

			inbox := message.NewInbox(app.db)
			var messages []*db.Message
			if unreadOnly {
				messages, err = inbox.ListUnread(cmd.Context(), agt.ID)
			} else {
				messages, err = inbox.ListMessages(cmd.Context(), agt.ID)
			}
			if err != nil {
				return fmt.Errorf("cli.newInboxCmd: list messages: %w", err)
			}

			filtered := make([]*db.Message, 0, len(messages))
			for _, msg := range messages {
				if fromFilter != "" && msg.FromAgent != fromFilter {
					continue
				}
				filtered = append(filtered, msg)
			}

			if jsonOutput {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				if err := enc.Encode(filtered); err != nil {
					return fmt.Errorf("cli.newInboxCmd: encode: %w", err)
				}
				return nil
			}

			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			if _, err := fmt.Fprintln(tw, "ID\tFROM\tTYPE\tTOPIC\tCREATED_AT\tREAD"); err != nil {
				return fmt.Errorf("cli.newInboxCmd: write header: %w", err)
			}
			for _, msg := range filtered {
				read := "unread"
				if msg.ReadAt != "" {
					read = "read"
				}
				if _, err := fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n", msg.ID, msg.FromAgent, msg.Type, msg.Topic, msg.CreatedAt, read); err != nil {
					return fmt.Errorf("cli.newInboxCmd: write row: %w", err)
				}
			}
			if err := tw.Flush(); err != nil {
				return fmt.Errorf("cli.newInboxCmd: flush: %w", err)
			}

			return nil
		},
	}

	f := cmd.Flags()
	f.StringVar(&agentName, "agent", "", "Agent name or ID")
	f.BoolVar(&unreadOnly, "unread", false, "Show only unread messages")
	f.StringVar(&fromFilter, "from", "", "Filter by sender agent ID")
	_ = cmd.MarkFlagRequired("agent")

	return cmd
}
