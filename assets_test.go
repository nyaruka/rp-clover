package clover

import (
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAssets(t *testing.T) {
	fs := Assets()

	f, err := fs.Open("/admin/index.html")
	require.NoError(t, err, "embedded admin/index.html should be openable via the path used by loadTemplate")
	defer f.Close()

	body, err := io.ReadAll(f)
	require.NoError(t, err)
	assert.Contains(t, string(body), "Clover Editor", "embedded index.html should contain the page title")
}
