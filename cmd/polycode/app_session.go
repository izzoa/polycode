package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"context"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/izzoa/polycode/internal/config"
	"github.com/izzoa/polycode/internal/provider"
	"github.com/izzoa/polycode/internal/tui"
)

// wireSessionHandlers sets up session-related handlers: auto-naming,
// session picker refresh, and the /sessions command.
func wireSessionHandlers(
	model *tui.Model,
	programRef **tea.Program,
	state *atomic.Pointer[appState],
) {
	model.SetAutoNameSessionHandler(func(prompt string, gen int) {
		go func() {
			program := *programRef
			s := state.Load()
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			// Truncate prompt for naming to avoid large requests
			namingInput := prompt
			if len(namingInput) > 500 {
				namingInput = namingInput[:500]
			}
			namingPrompt := "Summarize this conversation topic in 3-5 words. Reply with only the short name, nothing else:\n\n" + namingInput
			stream, err := s.primary.Query(ctx, []provider.Message{
				{Role: provider.RoleUser, Content: namingPrompt},
			}, provider.QueryOpts{MaxTokens: 30})
			if err != nil {
				return // silently fail
			}
			var name string
			for chunk := range stream {
				if chunk.Error != nil {
					return
				}
				name += chunk.Delta
			}
			name = strings.TrimSpace(name)
			if name != "" {
				// Save to session file
				session, _ := config.LoadSession()
				if session != nil && !session.UserNamed {
					session.SetName(name, false)
					_ = config.SaveSession(session)
				}
				program.Send(tui.SessionNameMsg{Name: name, Generation: gen})
			}
		}()
	})

	model.SetSessionPickerRefreshHandler(func() {
		go func() {
			program := *programRef
			sessions, err := config.ListSessions()
			program.Send(tui.SessionPickerMsg{
				Sessions: sessions,
				Error:    err,
			})
		}()
	})

	model.SetSessionsHandler(func(subcommand, args string) {
		go func() {
			program := *programRef
			switch subcommand {
			case "", "list":
				sessions, err := config.ListSessions()
				if err != nil {
					program.Send(tui.ConsensusChunkMsg{Delta: fmt.Sprintf("\nError: %v\n", err), Done: true})
					return
				}
				if len(sessions) == 0 {
					program.Send(tui.ConsensusChunkMsg{Delta: "\nNo saved sessions.\n", Done: true})
					return
				}
				var sb strings.Builder
				sb.WriteString("\nSaved sessions:\n\n")
				for _, s := range sessions {
					current := ""
					if s.IsCurrent {
						current = " ← current"
					}
					fmt.Fprintf(&sb, "  %-20s  %d exchanges  %s%s\n",
						s.Name, s.Exchanges,
						s.UpdatedAt.Format("Jan 02 15:04"), current)
				}
				sb.WriteString("\nUse /name <name> to name the current session.\n")
				program.Send(tui.ConsensusChunkMsg{Delta: sb.String(), Done: true})
			case "name":
				if args == "" {
					program.Send(tui.ConsensusChunkMsg{Delta: "\nUsage: /name <session-name>\n", Done: true})
					return
				}
				session, _ := config.LoadSession()
				if session == nil {
					session = &config.Session{}
				}
				session.SetName(args, true) // user-named, prevents auto-overwrite
				if err := config.SaveSession(session); err != nil {
					program.Send(tui.ConsensusChunkMsg{Delta: fmt.Sprintf("\nError: %v\n", err), Done: true})
					return
				}
				program.Send(tui.ConsensusChunkMsg{Delta: fmt.Sprintf("\nSession named %q.\n", args), Done: true})
			case "show":
				if args == "" {
					program.Send(tui.ConsensusChunkMsg{Delta: "\nUsage: /sessions show <name>\n", Done: true})
					return
				}
				s, _, err := config.LoadSessionByName(args)
				if err != nil {
					program.Send(tui.ConsensusChunkMsg{Delta: fmt.Sprintf("\nError: %v\n", err), Done: true})
					return
				}
				// Display session summary
				var sb strings.Builder
				fmt.Fprintf(&sb, "\nSession: %s (%d exchanges)\n", args, len(s.Exchanges))
				for i, ex := range s.Exchanges {
					prompt := ex.Prompt
					if len(prompt) > 60 {
						prompt = prompt[:57] + "..."
					}
					fmt.Fprintf(&sb, "  %d. %s\n", i+1, prompt)
				}
				program.Send(tui.ConsensusChunkMsg{Delta: sb.String(), Done: true})
			case "rename":
				// Rename any session by name (not just current)
				parts := strings.SplitN(args, " ", 2)
				if len(parts) < 2 {
					program.Send(tui.ConsensusChunkMsg{Delta: "\nUsage: /sessions rename <oldname> <newname>\n", Done: true})
					return
				}
				oldName, newName := parts[0], strings.TrimSpace(parts[1])
				s, path, err := config.LoadSessionByName(oldName)
				if err != nil {
					program.Send(tui.ConsensusChunkMsg{Delta: fmt.Sprintf("\nError: %v\n", err), Done: true})
					return
				}
				s.Name = newName
				data, _ := json.MarshalIndent(s, "", "  ")
				if err := os.WriteFile(path, data, 0600); err != nil {
					program.Send(tui.ConsensusChunkMsg{Delta: fmt.Sprintf("\nError saving: %v\n", err), Done: true})
					return
				}
				program.Send(tui.ConsensusChunkMsg{Delta: fmt.Sprintf("\nRenamed %q → %q.\n", oldName, newName), Done: true})
				// Refresh picker if open
				sessions, _ := config.ListSessions()
				program.Send(tui.SessionPickerMsg{Sessions: sessions})
			case "delete":
				if args == "" {
					program.Send(tui.ConsensusChunkMsg{Delta: "\nUsage: /sessions delete <name>\n", Done: true})
					return
				}
				// Guard: prevent deleting the current session
				currentSession, _ := config.LoadSession()
				if currentSession != nil && currentSession.Name == args {
					program.Send(tui.ConsensusChunkMsg{Delta: "\nCannot delete the current session.\n", Done: true})
					return
				}
				if err := config.DeleteSessionByName(args); err != nil {
					program.Send(tui.ConsensusChunkMsg{Delta: fmt.Sprintf("\nError: %v\n", err), Done: true})
					return
				}
				program.Send(tui.ConsensusChunkMsg{Delta: fmt.Sprintf("\nSession %q deleted.\n", args), Done: true})
				// Refresh picker if open
				sessions, _ := config.ListSessions()
				program.Send(tui.SessionPickerMsg{Sessions: sessions})
			default:
				program.Send(tui.ConsensusChunkMsg{Delta: "\nUsage: /sessions [list|show|delete|rename]\n       /name <session-name>\n", Done: true})
			}
		}()
	})
}

// toSessionMessages converts provider messages to the serializable session
// format, preserving tool call data and tool result metadata.
func toSessionMessages(msgs []provider.Message) []config.SessionMessage {
	out := make([]config.SessionMessage, 0, len(msgs))
	for _, m := range msgs {
		sm := config.SessionMessage{
			Role:    string(m.Role),
			Content: m.Content,
		}
		// Preserve tool calls on assistant messages
		for _, tc := range m.ToolCalls {
			sm.ToolCalls = append(sm.ToolCalls, config.ToolCallRecord{
				ID:        tc.ID,
				Name:      tc.Name,
				Arguments: tc.Arguments,
			})
		}
		// Preserve tool result metadata on tool role messages
		if m.ToolCallID != "" {
			sm.ToolResult = &config.ToolResultRecord{
				ToolCallID: m.ToolCallID,
				Output:     m.Content,
			}
		}
		out = append(out, sm)
	}
	return out
}

// fromSessionMessages converts serialized session messages back to provider
// messages, restoring tool call data and tool result metadata.
func fromSessionMessages(msgs []config.SessionMessage) []provider.Message {
	out := make([]provider.Message, 0, len(msgs))
	for _, sm := range msgs {
		m := provider.Message{
			Role:    provider.Role(sm.Role),
			Content: sm.Content,
		}
		// Restore tool calls
		for _, tc := range sm.ToolCalls {
			m.ToolCalls = append(m.ToolCalls, provider.ToolCall{
				ID:        tc.ID,
				Name:      tc.Name,
				Arguments: tc.Arguments,
			})
		}
		// Restore tool result ID and content from ToolResult
		if sm.ToolResult != nil {
			m.ToolCallID = sm.ToolResult.ToolCallID
			// Prefer ToolResult.Output if Content is empty (older sessions)
			if m.Content == "" && sm.ToolResult.Output != "" {
				m.Content = sm.ToolResult.Output
			}
		}
		out = append(out, m)
	}
	return out
}
