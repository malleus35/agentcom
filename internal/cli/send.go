package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/malleus35/agentcom/internal/agent"
	"github.com/malleus35/agentcom/internal/message"
	"github.com/malleus35/agentcom/internal/transport"
	"github.com/spf13/cobra"
)

func newSendCmd() *cobra.Command {
	var msgType string
	var topic string
	var correlationID string
	var from string

	cmd := &cobra.Command{
		Use:   "send <target> <message>",
		Short: "Send a message to one target agent",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			target := args[0]
			rawMessage := args[1]

			registry := agent.NewRegistry(app.db, app.cfg)
			sender, err := registry.FindByName(cmd.Context(), from)
			if err != nil {
				return fmt.Errorf("cli.newSendCmd: resolve sender: %w", err)
			}

			payload, err := buildPayload(rawMessage)
			if err != nil {
				return fmt.Errorf("cli.newSendCmd: parse payload: %w", err)
			}

			router := message.NewRouter(app.db, registry, transport.NewClient())
			env, err := router.Send(cmd.Context(), sender.ID, target, msgType, topic, payload)
			if err != nil {
				return fmt.Errorf("cli.newSendCmd: send: %w", err)
			}

			if correlationID != "" {
				if _, err := app.db.ExecContext(cmd.Context(), `UPDATE messages SET correlation_id = ? WHERE id = ?`, correlationID, env.ID); err != nil {
					return fmt.Errorf("cli.newSendCmd: set correlation id: %w", err)
				}
				env.CorrelationID = correlationID
			}

			if jsonOutput {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				if err := enc.Encode(env); err != nil {
					return fmt.Errorf("cli.newSendCmd: encode: %w", err)
				}
				return nil
			}

			_, err = fmt.Fprintf(cmd.OutOrStdout(), "sent %s to %s (id=%s type=%s topic=%s)\n", msgType, target, env.ID, env.Type, env.Topic)
			if err != nil {
				return fmt.Errorf("cli.newSendCmd: write output: %w", err)
			}

			return nil
		},
	}

	f := cmd.Flags()
	f.StringVar(&msgType, "type", "notification", "Message type: request|notification")
	f.StringVar(&topic, "topic", "", "Optional message topic")
	f.StringVar(&correlationID, "correlation-id", "", "Optional correlation ID")
	f.StringVar(&from, "from", "", "Sender agent name")
	_ = cmd.MarkFlagRequired("from")

	return cmd
}

func buildPayload(input string) (json.RawMessage, error) {
	trimmed := strings.TrimSpace(input)
	if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
		if !json.Valid([]byte(trimmed)) {
			return nil, fmt.Errorf("cli.buildPayload: invalid json payload")
		}
		return json.RawMessage(trimmed), nil
	}

	payload, err := json.Marshal(map[string]string{"text": input})
	if err != nil {
		return nil, fmt.Errorf("cli.buildPayload: %w", err)
	}

	return json.RawMessage(payload), nil
}
