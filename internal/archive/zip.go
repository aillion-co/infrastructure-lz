package archive

import (
	"archive/zip"
	"context"
	"fmt"
	"io"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"

	"github.com/aillion-co/infrastructure-lz/internal/generator/helm"
	"github.com/aillion-co/infrastructure-lz/internal/telemetry"
)

// WriteZip writes Helm chart file entries into a zip archive.
func WriteZip(ctx context.Context, w io.Writer, files []helm.FileEntry) error {
	_, span := telemetry.Tracer().Start(ctx, "archive.WriteZip")
	defer span.End()
	span.SetAttributes(attribute.Int("zip.file_count", len(files)))

	zw := zip.NewWriter(w)

	for _, f := range files {
		fw, err := zw.Create(f.Path)
		if err != nil {
			_ = zw.Close()
			span.RecordError(err)
			span.SetStatus(codes.Error, "creating zip entry")
			return fmt.Errorf("creating zip entry %s: %w", f.Path, err)
		}
		if _, err := fw.Write(f.Content); err != nil {
			_ = zw.Close()
			span.RecordError(err)
			span.SetStatus(codes.Error, "writing zip entry")
			return fmt.Errorf("writing zip entry %s: %w", f.Path, err)
		}
	}

	if err := zw.Close(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "closing zip writer")
		return fmt.Errorf("closing zip writer: %w", err)
	}
	return nil
}
