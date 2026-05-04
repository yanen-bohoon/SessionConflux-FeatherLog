package transport

import (
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// newTestSSHServer starts an in-process SSH+SFTP server on a random port,
// backed by an in-memory filesystem. Returns (addr, signer, cleanup).
func newTestSSHServer(t *testing.T) (string, ssh.Signer, func()) {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	signer, err := ssh.NewSignerFromKey(key)
	if err != nil {
		t.Fatalf("new signer: %v", err)
	}

	serverConfig := &ssh.ServerConfig{
		NoClientAuth: true,
	}
	serverConfig.AddHostKey(signer)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	done := make(chan struct{})

	go func() {
		defer close(done)
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		// Create a writable work dir for the SFTP server.
		workDir := t.TempDir()
		if err := os.MkdirAll(filepath.Join(workDir, "sessions"), 0755); err != nil {
			println("mkdir sessions:", err.Error())
				return
		}

		sshConn, chans, reqs, err := ssh.NewServerConn(conn, serverConfig)
		if err != nil {
			return
		}
		go ssh.DiscardRequests(reqs)

		for newChannel := range chans {
			if newChannel.ChannelType() != "session" {
				newChannel.Reject(ssh.UnknownChannelType, "unknown")
				continue
			}
			channel, requests, err := newChannel.Accept()
			if err != nil {
				continue
			}

			go func() {
				for req := range requests {
					ok := false
					if req.Type == "subsystem" && len(req.Payload) > 4 {
						subsystem := string(req.Payload[4:])
						if subsystem == "sftp" {
							ok = true
							go func() {
								server, err := sftp.NewServer(channel, sftp.WithServerWorkingDirectory(workDir))
								if err != nil {
									return
								}
								server.Serve()
							}()
						}
					}
					req.Reply(ok, nil)
					if !ok {
						return
					}
				}
			}()
		}
		sshConn.Close()
	}()

	cleanup := func() {
		listener.Close()
		<-done
	}
	return listener.Addr().String(), signer, cleanup
}

// Test helpers that construct SSHTransport with key auth for tests.
func newTestSSHTransport(addr string, signer ssh.Signer) *SSHTransport {
	return &SSHTransport{
		host:       "127.0.0.1",
		port:       mustParsePort(addr),
		user:       "test",
		remotePath: "sessions",
		signer:     signer,
	}
}

func mustParsePort(addr string) int {
	_, portStr, _ := net.SplitHostPort(addr)
	var port int
	fmt.Sscanf(portStr, "%d", &port)
	return port
}

func TestSSHTransport_Name(t *testing.T) {
	s := &SSHTransport{}
	if s.Name() != "ssh" {
		t.Errorf("Name = %q, want ssh", s.Name())
	}
}

func TestSSHTransport_CreateFolder(t *testing.T) {
	addr, signer, cleanup := newTestSSHServer(t)
	defer cleanup()

	tr := newTestSSHTransport(addr, signer)
	if err := tr.connect(); err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer tr.sftpClient.Close()
	defer tr.sshClient.Close()

	err := tr.CreateFolder("mac-studio/baseline")
	if err != nil {
		t.Fatalf("CreateFolder: %v", err)
	}

	// Verify by listing — folder should exist.
	files, err := tr.ListFiles("mac-studio")
	if err != nil {
		t.Fatalf("ListFiles after create: %v", err)
	}
	found := false
	for _, f := range files {
		if f.Name == "baseline" && f.IsDir {
			found = true
			break
		}
	}
	if !found {
		t.Error("baseline folder not found after create")
	}
}

func TestSSHTransport_UploadAndDownloadFile(t *testing.T) {
	addr, signer, cleanup := newTestSSHServer(t)
	defer cleanup()

	tr := newTestSSHTransport(addr, signer)
	if err := tr.connect(); err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer tr.sftpClient.Close()
	defer tr.sshClient.Close()

	// Create folder structure.
	if err := tr.CreateFolder("mac-studio/incremental"); err != nil {
		t.Fatalf("CreateFolder: %v", err)
	}

	// Upload.
	testData := []byte("hello from session-conflux")
	err := tr.UploadFile("mac-studio/incremental", "claude/sess-123.jsonl.zst", testData)
	if err != nil {
		t.Fatalf("UploadFile: %v", err)
	}

	// Download.
	data, err := tr.DownloadFile("mac-studio/incremental/claude/sess-123.jsonl.zst")
	if err != nil {
		t.Fatalf("DownloadFile: %v", err)
	}
	if string(data) != string(testData) {
		t.Errorf("data = %q, want %q", string(data), string(testData))
	}
}

func TestSSHTransport_DeleteFile(t *testing.T) {
	addr, signer, cleanup := newTestSSHServer(t)
	defer cleanup()

	tr := newTestSSHTransport(addr, signer)
	if err := tr.connect(); err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer tr.sftpClient.Close()
	defer tr.sshClient.Close()

	if err := tr.CreateFolder("host/incremental"); err != nil {
		t.Fatalf("CreateFolder: %v", err)
	}
	if err := tr.UploadFile("host/incremental", "sess.jsonl.zst", []byte("x")); err != nil {
		t.Fatalf("UploadFile: %v", err)
	}

	if err := tr.DeleteFile("host/incremental/sess.jsonl.zst"); err != nil {
		t.Fatalf("DeleteFile: %v", err)
	}

	// Verify gone.
	_, err := tr.DownloadFile("host/incremental/sess.jsonl.zst")
	if err == nil {
		t.Error("expected error after delete, got nil")
	}
}

func TestSSHTransport_DeleteFile_Idempotent(t *testing.T) {
	addr, signer, cleanup := newTestSSHServer(t)
	defer cleanup()

	tr := newTestSSHTransport(addr, signer)
	if err := tr.connect(); err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer tr.sftpClient.Close()
	defer tr.sshClient.Close()

	// Delete non-existent file should succeed.
	err := tr.DeleteFile("nonexistent/file.jsonl.zst")
	if err != nil {
		t.Fatalf("DeleteFile should be idempotent, got error: %v", err)
	}
}

func TestSSHTransport_ListFiles_Empty(t *testing.T) {
	addr, signer, cleanup := newTestSSHServer(t)
	defer cleanup()

	tr := newTestSSHTransport(addr, signer)
	if err := tr.connect(); err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer tr.sftpClient.Close()
	defer tr.sshClient.Close()

	// Create folder first so it exists.
	if err := tr.CreateFolder("empty-host"); err != nil {
		t.Fatalf("CreateFolder: %v", err)
	}

	files, err := tr.ListFiles("empty-host")
	if err != nil {
		t.Fatalf("ListFiles empty: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("got %d files in empty dir, want 0", len(files))
	}
}
