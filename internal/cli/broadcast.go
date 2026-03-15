package cli

import (
	"encoding/json"
	"fmt"

	"github.com/malleus35/agentcom/internal/agent"
	"github.com/malleus35/agentcom/internal/message"
	"github.com/malleus35/agentcom/internal/transport"
	"github.com/spf13/cobra"
)

func newBroadcastCmd() *cobra.Command {
	var topic string
	var from string

	cmd := &cobra.Command{
		Use:   "broadcast <message>",
		Short: "Broadcast a message to all alive agents",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			rawMessage := args[0]

			registry := agent.NewRegistry(app.db, app.cfg)
			sender, err := registry.FindByName(cmd.Context(), from, currentProjectFilter())
			if err != nil {
				return fmt.Errorf("cli.newBroadcastCmd: resolve sender: %w", err)
			}

			payload, err := buildPayload(rawMessage)
			if err != nil {
				return fmt.Errorf("cli.newBroadcastCmd: parse payload: %w", err)
			}

			router := message.NewRouter(app.db, registry, transport.NewClient(), currentProjectFilter())
			envelopes, err := router.Broadcast(cmd.Context(), sender.ID, topic, payload)
			if err != nil {
				return fmt.Errorf("cli.newBroadcastCmd: broadcast: %w", err)
			}

			if jsonOutput {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				if err := enc.Encode(map[string]any{
					"from":             sender.Name,
					"topic":            topic,
					"successful_sends": len(envelopes),
					"envelopes":        envelopes,
				}); err != nil {
					return fmt.Errorf("cli.newBroadcastCmd: encode: %w", err)
				}
				return nil
			}

			_, err = fmt.Fprintf(cmd.OutOrStdout(), "broadcast sent to %d agents\n", len(envelopes))
			if err != nil {
				return fmt.Errorf("cli.newBroadcastCmd: write output: %w", err)
			}

			return nil
		},
	}

	f := cmd.Flags()
	f.StringVar(&topic, "topic", "", "Optional broadcast topic")
	f.StringVar(&from, "from", "", "Sender agent name")
	_ = cmd.MarkFlagRequired("from")

	return cmd
}
