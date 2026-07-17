package parser

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseRooCodeSessionMistakeLimitPairing(t *testing.T) {
	tmpDir := t.TempDir()
	taskDir := filepath.Join(tmpDir, "tasks", "test-task-mistake")
	require.NoError(t, os.MkdirAll(taskDir, 0755))

	historyItem := rooCodeHistoryItem{
		ID:        "test-task-mistake",
		Number:    1,
		Timestamp: 1688836851000,
		Task:      "Mistake limit test",
		TokensIn:  10,
		TokensOut: 5,
		Workspace: "/Users/test/project",
	}
	historyJSON, err := json.Marshal(historyItem)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(
		filepath.Join(taskDir, "history_item.json"),
		historyJSON, 0644,
	))

	// Sequence: user task, command ask, mistake_limit_reached ask
	// mistake_limit_reached appears as an ask type in real RooCode data.
	messages := []rooCodeMessage{
		{
			Timestamp: 1688836851000,
			Type:      "say",
			Say:       "text",
			Text:      "Run the tests",
		},
		{
			Timestamp: 1688836860000,
			Type:      "ask",
			Ask:       "command",
			Text:      "npm test",
		},
		{
			Timestamp: 1688836870000,
			Type:      "ask",
			Ask:       "mistake_limit_reached",
			Text:      "Too many mistakes, stopping",
		},
	}
	messagesJSON, err := json.Marshal(messages)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(
		filepath.Join(taskDir, "ui_messages.json"),
		messagesJSON, 0644,
	))

	sess, msgs, err := parseRooCodeSession(taskDir, "", "")
	require.NoError(t, err)

	// user task + execute_command tool call = 2
	assert.Equal(t, 2, sess.MessageCount)

	// Verify the tool call has an errored ResultEvent.
	require.Len(t, msgs[1].ToolCalls, 1)
	tc := msgs[1].ToolCalls[0]
	assert.Equal(t, "execute_command", tc.ToolName)
	require.Len(t, tc.ResultEvents, 1)
	assert.Equal(t, "errored", tc.ResultEvents[0].Status)
	assert.Equal(t, "Too many mistakes, stopping", tc.ResultEvents[0].Content)
}

func TestParseRooCodeSessionAPIReqFailedPairing(t *testing.T) {
	tmpDir := t.TempDir()
	taskDir := filepath.Join(tmpDir, "tasks", "test-task-apifail")
	require.NoError(t, os.MkdirAll(taskDir, 0755))

	historyItem := rooCodeHistoryItem{
		ID:        "test-task-apifail",
		Number:    1,
		Timestamp: 1688836851000,
		Task:      "API req failed test",
		TokensIn:  10,
		TokensOut: 5,
		Workspace: "/Users/test/project",
	}
	historyJSON, err := json.Marshal(historyItem)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(
		filepath.Join(taskDir, "history_item.json"),
		historyJSON, 0644,
	))

	// Sequence: user task, MCP tool call, api_req_failed ask
	// api_req_failed appears as an ask type in real RooCode data.
	messages := []rooCodeMessage{
		{
			Timestamp: 1688836851000,
			Type:      "say",
			Say:       "text",
			Text:      "Search for docs",
		},
		{
			Timestamp: 1688836860000,
			Type:      "ask",
			Ask:       "use_mcp_server",
			Text:      `{"tool":"brave-search","serverName":"brave","query":"React"}`,
		},
		{
			Timestamp: 1688836870000,
			Type:      "ask",
			Ask:       "api_req_failed",
			Text:      "API request failed: 401 Unauthorized",
		},
	}
	messagesJSON, err := json.Marshal(messages)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(
		filepath.Join(taskDir, "ui_messages.json"),
		messagesJSON, 0644,
	))

	sess, msgs, err := parseRooCodeSession(taskDir, "", "")
	require.NoError(t, err)

	// user task + MCP tool call = 2
	assert.Equal(t, 2, sess.MessageCount)

	// Verify the MCP tool call has an errored ResultEvent.
	require.Len(t, msgs[1].ToolCalls, 1)
	tc := msgs[1].ToolCalls[0]
	assert.Equal(t, "brave-search", tc.ToolName)
	assert.Equal(t, "MCP", tc.Category)
	require.Len(t, tc.ResultEvents, 1)
	assert.Equal(t, "errored", tc.ResultEvents[0].Status)
	assert.Equal(t, "API request failed: 401 Unauthorized", tc.ResultEvents[0].Content)
}

func TestRooCodeIsToolErrorEvent(t *testing.T) {
	tests := []struct {
		say  string
		ask  string
		want bool
	}{
		{"mistake_limit_reached", "", true},
		{"api_req_failed", "", true},
		{"error", "", false},
		{"text", "", false},
		{"", "command", false},
		{"", "tool", false},
	}

	for _, tt := range tests {
		got := rooCodeIsToolErrorEvent(tt.say, tt.ask)
		assert.Equal(t, tt.want, got,
			"rooCodeIsToolErrorEvent(%q, %q) = %v, want %v",
			tt.say, tt.ask, got, tt.want)
	}
}
