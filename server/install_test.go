package server

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/Minenetpro/pelican-wings/config"
	"github.com/Minenetpro/pelican-wings/remote"
)

func TestInstallationProcessInstallerIdentity(t *testing.T) {
	cfg := &config.Configuration{AuthenticationToken: "test-token"}
	cfg.System.User.Uid = 1001
	cfg.System.User.Gid = 1002
	config.Set(cfg)

	ip := &InstallationProcess{}
	identity := ip.installerIdentity()

	if identity.containerUser != "1001:1002" {
		t.Fatalf("expected rootful container user 1001:1002, got %q", identity.containerUser)
	}
	if identity.scriptUID != 1001 || identity.scriptGID != 1002 {
		t.Fatalf("expected rootful script ownership 1001:1002, got %d:%d", identity.scriptUID, identity.scriptGID)
	}

	cfg = &config.Configuration{AuthenticationToken: "test-token"}
	cfg.System.User.Rootless.Enabled = true
	cfg.System.User.Rootless.ContainerUID = 0
	cfg.System.User.Rootless.ContainerGID = 0
	config.Set(cfg)

	identity = ip.installerIdentity()
	if identity.containerUser != "0:0" {
		t.Fatalf("expected rootless container user 0:0, got %q", identity.containerUser)
	}
	if identity.scriptUID != os.Getuid() || identity.scriptGID != os.Getgid() {
		t.Fatalf("expected rootless script ownership %d:%d, got %d:%d", os.Getuid(), os.Getgid(), identity.scriptUID, identity.scriptGID)
	}
}

func TestInstallationProcessWriteScriptToDisk(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.Configuration{AuthenticationToken: "test-token"}
	cfg.System.TmpDirectory = tmpDir
	cfg.System.User.Uid = os.Getuid()
	cfg.System.User.Gid = os.Getgid()
	config.Set(cfg)

	s, err := New(nil)
	if err != nil {
		t.Fatalf("failed to create test server: %v", err)
	}
	s.cfg.Uuid = "install-write-script"

	ip := &InstallationProcess{
		Server: s,
		Script: &remote.InstallationScript{
			Script: "#!/bin/sh\r\necho ready\r\n",
		},
	}

	if err := ip.writeScriptToDisk(); err != nil {
		t.Fatalf("writeScriptToDisk returned error: %v", err)
	}

	dirInfo, err := os.Stat(ip.tempDir())
	if err != nil {
		t.Fatalf("failed to stat temp directory: %v", err)
	}
	if got := dirInfo.Mode().Perm(); got != 0o700 {
		t.Fatalf("expected temp directory mode 0700, got %04o", got)
	}

	scriptPath := filepath.Join(ip.tempDir(), "install.sh")
	content, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatalf("failed to read staged install script: %v", err)
	}
	if got := string(content); got != "#!/bin/sh\necho ready\n" {
		t.Fatalf("unexpected staged script contents: %q", got)
	}

	scriptInfo, err := os.Stat(scriptPath)
	if err != nil {
		t.Fatalf("failed to stat staged install script: %v", err)
	}
	if got := scriptInfo.Mode().Perm(); got != 0o600 {
		t.Fatalf("expected staged script mode 0600, got %04o", got)
	}

	dirStat, ok := dirInfo.Sys().(*syscall.Stat_t)
	if !ok {
		t.Fatal("unexpected temp directory stat type")
	}
	if int(dirStat.Uid) != os.Getuid() || int(dirStat.Gid) != os.Getgid() {
		t.Fatalf("expected temp directory ownership %d:%d, got %d:%d", os.Getuid(), os.Getgid(), dirStat.Uid, dirStat.Gid)
	}

	fileStat, ok := scriptInfo.Sys().(*syscall.Stat_t)
	if !ok {
		t.Fatal("unexpected install script stat type")
	}
	if int(fileStat.Uid) != os.Getuid() || int(fileStat.Gid) != os.Getgid() {
		t.Fatalf("expected staged script ownership %d:%d, got %d:%d", os.Getuid(), os.Getgid(), fileStat.Uid, fileStat.Gid)
	}
}

func TestSetInstallScriptAccessFallsBackWhenOwnershipCannotBeChanged(t *testing.T) {
	dirPath := t.TempDir()
	scriptPath := filepath.Join(dirPath, "install.sh")
	if err := os.WriteFile(scriptPath, []byte("#!/bin/sh\necho ready\n"), 0o600); err != nil {
		t.Fatalf("failed writing staged install script: %v", err)
	}

	fallbackErr, err := setInstallScriptAccess(dirPath, scriptPath, installerIdentity{
		scriptUID: 1001,
		scriptGID: 1002,
	}, func(path string, uid, gid int) error {
		return &os.PathError{Op: "chown", Path: path, Err: syscall.EPERM}
	})
	if err != nil {
		t.Fatalf("setInstallScriptAccess returned fatal error: %v", err)
	}
	if fallbackErr == nil {
		t.Fatal("expected ownership failure to trigger readable fallback permissions")
	}

	dirInfo, err := os.Stat(dirPath)
	if err != nil {
		t.Fatalf("failed to stat temp directory: %v", err)
	}
	if got := dirInfo.Mode().Perm(); got != 0o755 {
		t.Fatalf("expected temp directory mode 0755 after fallback, got %04o", got)
	}

	scriptInfo, err := os.Stat(scriptPath)
	if err != nil {
		t.Fatalf("failed to stat staged install script: %v", err)
	}
	if got := scriptInfo.Mode().Perm(); got != 0o644 {
		t.Fatalf("expected staged script mode 0644 after fallback, got %04o", got)
	}
}
