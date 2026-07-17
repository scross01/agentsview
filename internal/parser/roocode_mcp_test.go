package parser

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseRooCodeSessionMCPResponsePairing(t *testing.T) {
	tmpDir := t.TempDir()
	taskDir := filepath.Join(tmpDir, "tasks", "test-task-mcp-pair")
	require.NoError(t, os.MkdirAll(taskDir, 0755))

	historyItem := rooCodeHistoryItem{
		ID:        "test-task-mcp-pair",
		Number:    1,
		Timestamp: 1688836851000,
		Task:      "MCP pairing test",
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

	// Sequence: user task, MCP tool call, mcp_server_response
	messages := []rooCodeMessage{
		{
			Timestamp: 1688836851000,
			Type:      "say",
			Say:       "text",
			Text:      "Search for documentation",
		},
		{
			Timestamp: 1688836860000,
			Type:      "ask",
			Ask:       "use_mcp_server",
			Text:      `{"tool":"brave-search","serverName":"brave","query":"React docs"}`,
		},
		{
			Timestamp: 1688836865000,
			Type:      "say",
			Say:       "mcp_server_request_started",
			Text:      "",
		},
		{
			Timestamp: 1688836870000,
			Type:      "say",
			Say:       "mcp_server_response",
			Text:      "Found React documentation at react.dev",
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

	// user task + MCP tool call = 2 (mcp_server_response is paired,
	// mcp_server_request_started is metadata/skipped)
	assert.Equal(t, 2, sess.MessageCount)

	// Verify the MCP tool call has a ResultEvent with the response.
	require.Len(t, msgs[1].ToolCalls, 1)
	tc := msgs[1].ToolCalls[0]
	assert.Equal(t, "brave-search", tc.ToolName)
	assert.Equal(t, "MCP", tc.Category)
	require.Len(t, tc.ResultEvents, 1)
	assert.Equal(t, "completed", tc.ResultEvents[0].Status)
	assert.Equal(t, "Found React documentation at react.dev",
		tc.ResultEvents[0].Content)
}

func TestParseRooCodeSessionMCPResponseNoPending(t *testing.T) {
	tmpDir := t.TempDir()
	taskDir := filepath.Join(tmpDir, "tasks", "test-task-mcp-nopending")
	require.NoError(t, os.MkdirAll(taskDir, 0755))

	historyItem := rooCodeHistoryItem{
		ID:        "test-task-mcp-nopending",
		Number:    1,
		Timestamp: 1688836851000,
		Task:      "MCP no pending test",
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

	// MCP response with no preceding MCP tool call — should be
	// a standalone system message.
	messages := []rooCodeMessage{
		{
			Timestamp: 1688836851000,
			Type:      "say",
			Say:       "text",
			Text:      "Hello",
		},
		{
			Timestamp: 1688836860000,
			Type:      "say",
			Say:       "mcp_server_response",
			Text:      "Orphaned MCP response",
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

	// user task + MCP response system msg = 2
	assert.Equal(t, 2, sess.MessageCount)

	// Response should be a standalone system message.
	assert.Equal(t, RoleSystem, msgs[1].Role)
	assert.True(t, msgs[1].IsSystem)
	assert.Equal(t, "Orphaned MCP response", msgs[1].Content)
}

func TestRooCodeIsMetadataSayIncludesMCP(t *testing.T) {
	assert.True(t, rooCodeIsMetadataSay("mcp_server_request_started"),
		"mcp_server_request_started should be treated as metadata")
}
