package sync

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.kenn.io/agentsview/internal/parser"
)

// TestSourceMtimeRooCodeUsesProviderFingerprint confirms that the
// session-watch fallback observes ui_messages.json updates. RooCode
// freshness spans both history_item.json and ui_messages.json, so a
// plain stat of the stored history_item.json path would miss transcript
// updates. SourceMtime must route through the provider fingerprint,
// which composes both files, and return the newer ui_messages.json mtime.
func TestSourceMtimeRooCodeUsesProviderFingerprint(t *testing.T) {
	database := openTestDB(t)
	root := t.TempDir()
	engine := NewEngine(database, EngineConfig{
		AgentDirs: map[parser.AgentType][]string{
			parser.AgentRooCode: {root},
		},
		Machine: "local",
	})

	rawID := "mtime-task"
	taskDir := filepath.Join(root, "tasks", rawID)
	require.NoError(t, os.MkdirAll(taskDir, 0o755))

	historyPath := filepath.Join(taskDir, "history_item.json")
	messagesPath := filepath.Join(taskDir, "ui_messages.json")
	require.NoError(t, os.WriteFile(historyPath, []byte(
		`{"id":"mtime-task","number":1,"ts":1688836851000,`+
			`"task":"Test task","tokensIn":10,"tokensOut":20,`+
			`"workspace":"/tmp/roocode","mode":"code","status":"completed"}`,
	), 0o644))
	require.NoError(t, os.WriteFile(messagesPath, []byte(
		`[{"ts":1688836851000,"type":"say","say":"text","text":"Test task"}]`,
	), 0o644))

	historyTime := time.Date(2026, time.June, 4, 10, 0, 0, 0, time.UTC)
	messagesTime := historyTime.Add(5 * time.Minute)
	require.NoError(t, os.Chtimes(historyPath, historyTime, historyTime))
	require.NoError(t, os.Chtimes(messagesPath, messagesTime, messagesTime))

	assert.Equal(
		t,
		messagesTime.UnixNano(),
		engine.SourceMtime("roocode:"+rawID),
		"SourceMtime must return the newer ui_messages.json mtime",
	)
}
