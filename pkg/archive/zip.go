package archive

import (
	"archive/zip"
	"fmt"
	"io"

	"github.com/aillion-co/infrastructure-lz/internal/generator/helm"
)

// WriteZip writes Helm chart file entries into a zip archive.
func WriteZip(w io.Writer, files []helm.FileEntry) error {
	zw := zip.NewWriter(w)

	for _, f := range files {
		fw, err := zw.Create(f.Path)
		if err != nil {
			_ = zw.Close()
			return fmt.Errorf("creating zip entry %s: %w", f.Path, err)
		}
		if _, err := fw.Write(f.Content); err != nil {
			_ = zw.Close()
			return fmt.Errorf("writing zip entry %s: %w", f.Path, err)
		}
	}

	if err := zw.Close(); err != nil {
		return fmt.Errorf("closing zip writer: %w", err)
	}
	return nil
}
