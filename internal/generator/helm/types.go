package helm

// FileEntry represents a file in the Helm chart output.
type FileEntry struct {
	Path    string
	Content []byte
}
