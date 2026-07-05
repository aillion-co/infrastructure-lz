package archive_test

import (
	"archive/zip"
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aillion-co/infrastructure-lz/internal/archive"
	"github.com/aillion-co/infrastructure-lz/internal/generator/helm"
)

func TestWriteZip_RoundTrip_PreservesFiles(t *testing.T) {
	files := []helm.FileEntry{
		{Path: "chart/Chart.yaml", Content: []byte("apiVersion: v2\nname: test\n")},
		{Path: "chart/values.yaml", Content: []byte("enabled: true\n")},
		{Path: "chart/templates/empty.yaml", Content: nil},
	}

	var buf bytes.Buffer
	require.NoError(t, archive.WriteZip(context.Background(), &buf, files))

	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	require.NoError(t, err)
	require.Len(t, zr.File, len(files))

	got := map[string][]byte{}
	for _, zf := range zr.File {
		rc, err := zf.Open()
		require.NoError(t, err)
		var content bytes.Buffer
		_, err = content.ReadFrom(rc)
		require.NoError(t, err)
		require.NoError(t, rc.Close())
		got[zf.Name] = content.Bytes()
	}

	for _, f := range files {
		assert.Equal(t, string(f.Content), string(got[f.Path]), "content mismatch for %s", f.Path)
	}
}

func TestWriteZip_NoFiles_ValidEmptyArchive(t *testing.T) {
	var buf bytes.Buffer
	require.NoError(t, archive.WriteZip(context.Background(), &buf, nil))

	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	require.NoError(t, err)
	assert.Empty(t, zr.File)
}
