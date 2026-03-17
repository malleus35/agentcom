package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/malleus35/agentcom/internal/db"
	"github.com/malleus35/agentcom/internal/message"
	"github.com/malleus35/agentcom/internal/transport"
	"github.com/spf13/cobra"
)

type userMessageView struct {
	ID            string          `json:"id"`
	FromAgentID   string          `json:"from_agent_id"`
	FromAgentName string          `json:"from_agent_name"`
	ToAgentID     string          `json:"to_agent_id,omitempty"`
	Type          string          `json:"type"`
	Topic         string          `json:"topic,omitempty"`
	Payload       json.RawMessage `json:"payload"`
	CreatedAt     string          `json:"created_at"`
	Read          bool            `json:"read"`
	ReadAt        string          `json:"read_at,omitempty"`
}

func newUserCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "user",
		Short: "Interact with the project user inbox",
	}
	cmd.AddCommand(newUserInboxCmd(), newUserReplyCmd(), newUserPendingCmd())
	return cmd
}

func newUserInboxCmd() *cobra.Command {
	var unreadOnly bool
	var fromFilter string

	cmd := &cobra.Command{
		Use:   "inbox",
		Short: "View messages addressed to the user agent",
		RunE: func(cmd *cobra.Command, args []string) error {
			userAgent, err := resolveUserAgent(cmd.Context(), app.db, currentProjectFilter())
			if err != nil {
				return fmt.Errorf("cli.newUserInboxCmd: %w", err)
			}

			inbox := message.NewInbox(app.db)
			var messages []*db.Message
			if unreadOnly {
				messages, err = inbox.ListUnread(cmd.Context(), userAgent.ID)
			} else {
				messages, err = inbox.ListMessages(cmd.Context(), userAgent.ID)
			}
			if err != nil {
				return fmt.Errorf("cli.newUserInboxCmd: list messages: %w", err)
			}

			filtered, err := filterUserMessages(cmd.Context(), messages, fromFilter, currentProjectFilter())
			if err != nil {
				return fmt.Errorf("cli.newUserInboxCmd: filter messages: %w", err)
			}
			views, err := buildUserMessageViews(cmd.Context(), filtered)
			if err != nil {
				return fmt.Errorf("cli.newUserInboxCmd: build views: %w", err)
			}

			if err := markMessagesRead(cmd.Context(), filtered); err != nil {
				return fmt.Errorf("cli.newUserInboxCmd: mark read: %w", err)
			}

			if jsonOutput {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(views)
			}

			return writeUserMessageTable(cmd, views)
		},
	}

	f := cmd.Flags()
	f.BoolVar(&unreadOnly, "unread", true, "Show only unread messages")
	f.StringVar(&fromFilter, "from", "", "Filter by sender agent name or ID")
	return cmd
}

func newUserReplyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reply <target-agent> <message>",
		Short: "Send a response from the user agent",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			userAgent, err := resolveUserAgent(cmd.Context(), app.db, currentProjectFilter())
			if err != nil {
				return fmt.Errorf("cli.newUserReplyCmd: %w", err)
			}

			payload, err := buildPayload(args[1])
			if err != nil {
				return fmt.Errorf("cli.newUserReplyCmd: parse payload: %w", err)
			}

			router := message.NewRouter(app.db, appDBFinder{db: app.db}, transport.NewClient(), currentProjectFilter())
			env, err := router.Send(cmd.Context(), userAgent.ID, args[0], "response", "", payload)
			if err != nil {
				return fmt.Errorf("cli.newUserReplyCmd: send: %w", err)
			}

			if jsonOutput {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(env)
			}

			_, err = fmt.Fprintf(cmd.OutOrStdout(), "replied to %s (id=%s)\n", args[0], env.ID)
			return err
		},
	}
	return cmd
}

func newUserPendingCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pending",
		Short: "Show unread request messages waiting for the user",
		RunE: func(cmd *cobra.Command, args []string) error {
			userAgent, err := resolveUserAgent(cmd.Context(), app.db, currentProjectFilter())
			if err != nil {
				return fmt.Errorf("cli.newUserPendingCmd: %w", err)
			}

			messages, err := app.db.ListUnreadRequestsForAgent(cmd.Context(), userAgent.ID)
			if err != nil {
				return fmt.Errorf("cli.newUserPendingCmd: list pending: %w", err)
			}
			views, err := buildUserMessageViews(cmd.Context(), messages)
			if err != nil {
				return fmt.Errorf("cli.newUserPendingCmd: build views: %w", err)
			}

			if jsonOutput {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(views)
			}

			return writeUserMessageTable(cmd, views)
		},
	}
	return cmd
}

func resolveUserAgent(ctx context.Context, database *db.DB, project string) (*db.Agent, error) {
	userAgent, err := database.FindAgentByNameAndProject(ctx, "user", project)
	if err == nil {
		return userAgent, nil
	}
	if !errors.Is(err, db.ErrAgentNotFound) {
		return nil, fmt.Errorf("resolve user agent by name: %w", err)
	}

	userAgent, err = database.FindAgentByTypeAndProject(ctx, "human", project)
	if err == nil {
		return userAgent, nil
	}
	if errors.Is(err, db.ErrAgentNotFound) {
		return nil, fmt.Errorf("no user agent registered; run `agentcom up` first")
	}
	return nil, fmt.Errorf("resolve user agent by type: %w", err)
}

func filterUserMessages(ctx context.Context, messages []*db.Message, fromFilter string, project string) ([]*db.Message, error) {
	if strings.TrimSpace(fromFilter) == "" {
		return messages, nil
	}

	sender, err := appDBFinder{db: app.db}.FindByName(ctx, fromFilter, project)
	if err != nil {
		sender, err = appDBFinder{db: app.db}.FindByID(ctx, fromFilter)
		if err != nil {
			return nil, fmt.Errorf("resolve from filter: %w", err)
		}
	}

	filtered := make([]*db.Message, 0, len(messages))
	for _, msg := range messages {
		if msg.FromAgent == sender.ID {
			filtered = append(filtered, msg)
		}
	}
	return filtered, nil
}

func buildUserMessageViews(ctx context.Context, messages []*db.Message) ([]userMessageView, error) {
	views := make([]userMessageView, 0, len(messages))
	for _, msg := range messages {
		payload := json.RawMessage(msg.Payload)
		if len(payload) == 0 {
			payload = json.RawMessage(`{}`)
		}
		fromName, err := resolveAgentDisplayName(ctx, msg.FromAgent)
		if err != nil {
			return nil, err
		}
		views = append(views, userMessageView{
			ID:            msg.ID,
			FromAgentID:   msg.FromAgent,
			FromAgentName: fromName,
			ToAgentID:     msg.ToAgent,
			Type:          msg.Type,
			Topic:         msg.Topic,
			Payload:       payload,
			CreatedAt:     msg.CreatedAt,
			Read:          msg.ReadAt != "",
			ReadAt:        msg.ReadAt,
		})
	}
	return views, nil
}

func resolveAgentDisplayName(ctx context.Context, agentID string) (string, error) {
	if agentID == "" {
		return "", nil
	}
	agentRecord, err := app.db.FindAgentByID(ctx, agentID)
	if err == nil {
		return agentRecord.Name, nil
	}
	if errors.Is(err, db.ErrAgentNotFound) {
		return agentID, nil
	}
	return "", fmt.Errorf("resolve agent display name: %w", err)
}

func markMessagesRead(ctx context.Context, messages []*db.Message) error {
	for _, msg := range messages {
		if msg.ReadAt != "" {
			continue
		}
		if err := app.db.MarkRead(ctx, msg.ID); err != nil {
			return err
		}
	}
	return nil
}

func writeUserMessageTable(cmd *cobra.Command, views []userMessageView) error {
	tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintln(tw, "ID\tFROM\tTYPE\tTOPIC\tPAYLOAD\tCREATED_AT\tREAD"); err != nil {
		return fmt.Errorf("write header: %w", err)
	}
	for _, view := range views {
		read := "unread"
		if view.Read {
			read = "read"
		}
		if _, err := fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n", view.ID, view.FromAgentName, view.Type, view.Topic, payloadPreview(view.Payload), view.CreatedAt, read); err != nil {
			return fmt.Errorf("write row: %w", err)
		}
	}
	if err := tw.Flush(); err != nil {
		return fmt.Errorf("flush: %w", err)
	}
	return nil
}

func payloadPreview(payload json.RawMessage) string {
	preview := strings.TrimSpace(string(payload))
	if len(preview) <= 48 {
		return preview
	}
	return preview[:45] + "..."
}

type appDBFinder struct {
	db *db.DB
}

func (f appDBFinder) FindByName(ctx context.Context, name string, project string) (*db.Agent, error) {
	return f.db.FindAgentByNameAndProject(ctx, name, project)
}

func (f appDBFinder) FindByID(ctx context.Context, id string) (*db.Agent, error) {
	return f.db.FindAgentByID(ctx, id)
}

func (f appDBFinder) ListAlive(ctx context.Context, project string) ([]*db.Agent, error) {
	return f.db.ListAliveAgentsByProject(ctx, project)
}
