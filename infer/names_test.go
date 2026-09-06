package infer

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCanonicalName(t *testing.T) {
	t.Parallel()
	for input, want := range map[string]string{
		"server-port": "server_port", "serverPort": "server_port", "ServerPort": "server_port", "server.port": "server_port", "HTTPServer": "http_server", "databaseURL": "database_url", "server_port": "server_port", "server__port": "server__port", "123port": "field_123port", " café ": "caf_ue9", ".": "field",
	} {
		t.Run(input, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, want, canonicalName(input))
		})
	}
}
