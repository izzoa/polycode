package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/izzoa/polycode/internal/config"
	"github.com/spf13/cobra"
)

func runExport(cmd *cobra.Command, args []string) error {
	format, _ := cmd.Flags().GetString("format")
	output, _ := cmd.Flags().GetString("output")

	session, err := config.LoadSession()
	if err != nil {
		return fmt.Errorf("loading session: %w", err)
	}
	if session == nil || len(session.Exchanges) == 0 {
		return fmt.Errorf("no session to export — start a conversation first")
	}

	var content string
	switch format {
	case "json":
		data, err := json.MarshalIndent(session, "", "  ")
		if err != nil {
			return fmt.Errorf("marshaling session: %w", err)
		}
		content = string(data)
	case "md":
		content = FormatSessionMarkdown(session)
	default:
		return fmt.Errorf("unsupported format %q — use md or json", format)
	}

	if output != "" {
		if err := os.WriteFile(output, []byte(content), 0644); err != nil {
			return fmt.Errorf("writing output file: %w", err)
		}
		fmt.Printf("Session exported to %s\n", output)
	} else {
		fmt.Print(content)
	}

	return nil
}

func runImport(cmd *cobra.Command, args []string) error {
	filePath := args[0]

	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("reading import file: %w", err)
	}

	var session config.Session
	if err := json.Unmarshal(data, &session); err != nil {
		return fmt.Errorf("parsing session file: %w", err)
	}

	if err := config.SaveSession(&session); err != nil {
		return fmt.Errorf("saving session: %w", err)
	}

	fmt.Println("Session imported. Run `polycode` to resume.")
	return nil
}

// FormatSessionMarkdown renders a Session as a readable markdown document.
func FormatSessionMarkdown(s *config.Session) string {
	var b strings.Builder

	b.WriteString("# Polycode Session Export\n\n")
	b.WriteString(fmt.Sprintf("*Exported: %s*\n\n", time.Now().Format("2006-01-02")))

	for i, ex := range s.Exchanges {
		b.WriteString(fmt.Sprintf("## Turn %d\n\n", i+1))
		b.WriteString(fmt.Sprintf("**User:** %s\n\n", ex.Prompt))
		b.WriteString(fmt.Sprintf("**Consensus:** %s\n\n", ex.ConsensusResponse))

		if len(ex.Individual) > 0 {
			b.WriteString("**Individual Responses:**\n\n")
			for name, resp := range ex.Individual {
				// Truncate long individual responses for readability.
				summary := resp
				if len(summary) > 200 {
					summary = summary[:197] + "..."
				}
				b.WriteString(fmt.Sprintf("- *%s:* %s\n", name, summary))
			}
			b.WriteString("\n")
		}

		b.WriteString("---\n\n")
	}

	return b.String()
}
