//go:build linux

package cases

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"wsfs-core-mount-blackbox/harness"

	"golang.org/x/sys/unix"
)

const xattrTestKey = "user.wsfs_test.payload"

func xattrRegularFile(_ context.Context, env *harness.Env) error {
	path := filepath.Join(env.MountDir, "xattr.txt")
	if err := os.WriteFile(path, []byte("xattr target\n"), 0o644); err != nil {
		return err
	}

	initial := bytes.Repeat([]byte("a"), 1024)
	if err := syscall.Setxattr(path, xattrTestKey, initial, 0); err != nil {
		return fmt.Errorf("set xattr: %w", err)
	}
	if err := syscall.Setxattr(path, xattrTestKey, []byte("duplicate"), unix.XATTR_CREATE); !errors.Is(err, syscall.EEXIST) {
		return fmt.Errorf("create existing xattr: got %v, want EEXIST", err)
	}

	replaced := bytes.Repeat([]byte("b"), 768)
	if err := syscall.Setxattr(path, xattrTestKey, replaced, unix.XATTR_REPLACE); err != nil {
		return fmt.Errorf("replace xattr: %w", err)
	}
	got := make([]byte, len(replaced))
	n, err := syscall.Getxattr(path, xattrTestKey, got)
	if err != nil {
		return fmt.Errorf("get xattr: %w", err)
	}
	if !bytes.Equal(got[:n], replaced) {
		return fmt.Errorf("xattr value mismatch")
	}

	listSize, err := syscall.Listxattr(path, nil)
	if err != nil {
		return fmt.Errorf("list xattr size: %w", err)
	}
	list := make([]byte, listSize)
	if _, err := syscall.Listxattr(path, list); err != nil {
		return fmt.Errorf("list xattr: %w", err)
	}
	if !containsXattrName(list, xattrTestKey) {
		return fmt.Errorf("xattr list does not contain %q", xattrTestKey)
	}

	if err := syscall.Removexattr(path, xattrTestKey); err != nil {
		return fmt.Errorf("remove xattr: %w", err)
	}
	if _, err := syscall.Getxattr(path, xattrTestKey, got); !errors.Is(err, syscall.ENODATA) {
		return fmt.Errorf("get removed xattr: got %v, want ENODATA", err)
	}
	return nil
}

func verifyStorageXattrRegularFile(_ context.Context, env *harness.Env) error {
	path := filepath.Join(env.BackendDir, "xattr.txt")
	if _, err := syscall.Getxattr(path, xattrTestKey, make([]byte, 1)); !errors.Is(err, syscall.ENODATA) {
		return fmt.Errorf("backend get removed xattr: got %v, want ENODATA", err)
	}
	return nil
}

func containsXattrName(data []byte, want string) bool {
	for len(data) > 0 {
		end := bytes.IndexByte(data, 0)
		if end < 0 {
			return false
		}
		if string(data[:end]) == want {
			return true
		}
		data = data[end+1:]
	}
	return false
}

func init() {
	cases = append(cases, testCase{name: "xattr_regular_file", run: xattrRegularFile, verifyStorage: verifyStorageXattrRegularFile})
}
