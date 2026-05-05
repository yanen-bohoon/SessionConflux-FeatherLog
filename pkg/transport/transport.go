package transport

// FileInfo holds basic file metadata from a transport backend.
type FileInfo struct {
	Name  string
	IsDir bool
	Size  int64
}

// Transport abstracts file storage operations across backends
// (Feishu Drive, SSH/SFTP, NAS, S3, etc.).
//
// All paths use "/" as separator and are relative to the transport root
// (no leading slash). The root itself is backend-defined:
//
//	feishu: the "SessionConflux" Drive folder
//	ssh:    the configured remote_path
//
// Implementations must be safe for concurrent use.
type Transport interface {
	// CreateFolder creates a folder at path and any intermediate folders.
	// Idempotent — succeeds if the folder already exists.
	CreateFolder(path string) error

	// ListFiles lists children of path. Pass "" or "." for the root.
	ListFiles(path string) ([]FileInfo, error)

	// UploadFile writes data as a file named fileName into folderPath.
	// fileName may contain "/" to nest under subdirectories.
	UploadFile(folderPath, fileName string, data []byte) error

	// DownloadFile reads the file at path and returns its contents.
	DownloadFile(path string) ([]byte, error)

	// DeleteFile removes the file at path.
	// Idempotent — succeeds if the file is already gone.
	DeleteFile(path string) error

	// Name returns a short backend identifier: "feishu", "ssh", etc.
	Name() string

	// MaxChunkSize returns the maximum file size (in bytes) the backend
	// can upload as a single file. Returns 0 if there is no limit
	// and chunking is unnecessary. Backends with upload limits (e.g.
	// Feishu Drive's 20 MB) return the limit so the sync layer can
	// split large files accordingly.
	MaxChunkSize() int64

	// Verify checks that credentials and connectivity are valid.
	// Used by setup wizards to validate user-provided credentials
	// before saving the config.
	Verify() error
}
