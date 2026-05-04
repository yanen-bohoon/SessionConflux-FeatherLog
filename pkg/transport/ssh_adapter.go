package transport

import (
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"

	"github.com/yanen-bohoon/session-conflux/pkg/config"
)

// SSHTransport implements Transport via SFTP to a remote server.
type SSHTransport struct {
	host       string
	port       int
	user       string
	keyPath    string
	remotePath string

	// signer is set directly in tests to bypass key file loading.
	signer ssh.Signer

	sshClient  *ssh.Client
	sftpClient *sftp.Client
}

// NewSSHTransport creates an SSHTransport from config.
func NewSSHTransport(cfg config.SSHConfig) (*SSHTransport, error) {
	keyPath := cfg.KeyFile
	if strings.HasPrefix(keyPath, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("home dir: %w", err)
		}
		keyPath = filepath.Join(home, keyPath[2:])
	}
	port := cfg.Port
	if port == 0 {
		port = 22
	}
	t := &SSHTransport{
		host:       cfg.Host,
		port:       port,
		user:       cfg.User,
		keyPath:    keyPath,
		remotePath: strings.TrimRight(cfg.RemotePath, "/"),
	}
	if err := t.connect(); err != nil {
		return nil, err
	}
	return t, nil
}

func (t *SSHTransport) Name() string { return "ssh" }

// Verify checks that the SSH connection is still alive.
func (t *SSHTransport) Verify() error {
	if t.sftpClient == nil || t.sshClient == nil {
		return fmt.Errorf("ssh: not connected")
	}
	_, err := t.sftpClient.Getwd()
	return err
}

func (t *SSHTransport) connect() error {
	var authMethods []ssh.AuthMethod
	if t.signer != nil {
		authMethods = append(authMethods, ssh.PublicKeys(t.signer))
	} else {
		keyBytes, err := os.ReadFile(t.keyPath)
		if err != nil {
			return fmt.Errorf("read SSH key %q: %w", t.keyPath, err)
		}
		signer, err := ssh.ParsePrivateKey(keyBytes)
		if err != nil {
			return fmt.Errorf("parse SSH key: %w", err)
		}
		authMethods = append(authMethods, ssh.PublicKeys(signer))
	}

	config := &ssh.ClientConfig{
		User:            t.user,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}
	addr := net.JoinHostPort(t.host, fmt.Sprintf("%d", t.port))
	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return fmt.Errorf("ssh dial %s: %w", addr, err)
	}
	sftpClient, err := sftp.NewClient(client)
	if err != nil {
		client.Close()
		return fmt.Errorf("sftp init: %w", err)
	}
	t.sshClient = client
	t.sftpClient = sftpClient
	return nil
}

// Close closes the underlying SSH and SFTP connections.
func (t *SSHTransport) Close() error {
	var errs []error
	if t.sftpClient != nil {
		if err := t.sftpClient.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if t.sshClient != nil {
		if err := t.sshClient.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("close: %v", errs)
	}
	return nil
}

func (t *SSHTransport) fullPath(p string) string {
	p = strings.TrimLeft(p, "/")
	if p == "" || p == "." {
		return t.remotePath
	}
	return t.remotePath + "/" + p
}

// CreateFolder creates a folder tree. Idempotent.
func (t *SSHTransport) CreateFolder(path string) error {
	full := t.fullPath(path)
	if err := t.sftpClient.MkdirAll(full); err != nil {
		return fmt.Errorf("mkdir %s: %w", path, err)
	}
	return nil
}

// ListFiles lists children of path. Pass "" for root.
func (t *SSHTransport) ListFiles(path string) ([]FileInfo, error) {
	full := t.fullPath(path)
	entries, err := t.sftpClient.ReadDir(full)
	if err != nil {
		return nil, fmt.Errorf("list %s: %w", path, err)
	}
	out := make([]FileInfo, 0, len(entries))
	for _, e := range entries {
		out = append(out, FileInfo{
			Name:  e.Name(),
			IsDir: e.IsDir(),
			Size:  e.Size(),
		})
	}
	return out, nil
}

// UploadFile writes data as a file named fileName into folderPath.
func (t *SSHTransport) UploadFile(folderPath, fileName string, data []byte) error {
	dir := t.fullPath(folderPath)
	targetPath := filepath.Join(dir, fileName)
	// Ensure parent directory exists.
	if err := t.sftpClient.MkdirAll(filepath.Dir(targetPath)); err != nil {
		return fmt.Errorf("mkdir parent %q: %w", filepath.Dir(targetPath), err)
	}
	f, err := t.sftpClient.Create(targetPath)
	if err != nil {
		return fmt.Errorf("create %q: %w", targetPath, err)
	}
	defer f.Close()
	if _, err := f.Write(data); err != nil {
		return fmt.Errorf("write %q: %w", targetPath, err)
	}
	return nil
}

// DownloadFile reads the file at path.
func (t *SSHTransport) DownloadFile(path string) ([]byte, error) {
	full := t.fullPath(path)
	f, err := t.sftpClient.Open(full)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()
	return io.ReadAll(f)
}

// DeleteFile removes a file. Idempotent.
func (t *SSHTransport) DeleteFile(path string) error {
	full := t.fullPath(path)
	err := t.sftpClient.Remove(full)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("delete %s: %w", path, err)
	}
	return nil
}
