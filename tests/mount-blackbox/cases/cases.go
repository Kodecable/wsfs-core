package cases

import (
	"bytes"
	"context"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"

	"wsfs-core-mount-blackbox/harness"
)

type testCase struct {
	name    string
	prepare func(*harness.Env) error
	run     func(context.Context, *harness.Env) error
}

func (c testCase) Name() string { return c.name }
func (c testCase) Prepare(env *harness.Env) error {
	if c.prepare == nil {
		return nil
	}
	return c.prepare(env)
}
func (c testCase) Run(ctx context.Context, env *harness.Env) error {
	return c.run(ctx, env)
}

func All() []harness.Case {
	return []harness.Case{
		testCase{name: "create_write_read_small", run: createWriteReadSmall},
		testCase{name: "overwrite_existing_file", run: overwriteExistingFile},
		testCase{name: "append_via_std_write", run: appendViaStdWrite},
		testCase{name: "truncate_shrink", run: truncateShrink},
		testCase{name: "truncate_expand", run: truncateExpand},
		testCase{name: "getattr_after_write", run: getattrAfterWrite},
		testCase{name: "getattr_after_rename", run: getattrAfterRename},
		testCase{name: "read_at_offsets", prepare: prepareReadAtOffsets, run: readAtOffsets},
		testCase{name: "write_at_offsets", run: writeAtOffsets},
		testCase{name: "read_large_file_cross_message_boundary", prepare: prepareReadLargeFile, run: readLargeFileCrossMessageBoundary},
		testCase{name: "readdir_root", prepare: prepareReaddirRoot, run: readdirRoot},
		testCase{name: "create_many_entries", run: createManyEntries},
		testCase{name: "mkdir_then_readdir", run: mkdirThenReaddir},
		testCase{name: "rename_file_cross_dir", run: renameFileCrossDir},
		testCase{name: "rename_then_readdir_old_parent", run: renameThenReaddirOldParent},
		testCase{name: "remove_then_readdir", run: removeThenReaddir},
		testCase{name: "create_and_read_symlink", run: createAndReadSymlink},
		testCase{name: "read_file_via_symlink", run: readFileViaSymlink},
		testCase{name: "many_small_random_writes", run: manySmallRandomWrites},
		testCase{name: "write_large_file_cross_message_boundary", run: writeLargeFileCrossMessageBoundary},
	}
}

func Lookup(names []string) ([]harness.Case, error) {
	all := All()
	if len(names) == 0 {
		return all, nil
	}

	byName := map[string]harness.Case{}
	for _, c := range all {
		byName[c.Name()] = c
	}

	selected := make([]harness.Case, 0, len(names))
	for _, name := range names {
		c, ok := byName[name]
		if !ok {
			return nil, fmt.Errorf("unknown case %q", name)
		}
		selected = append(selected, c)
	}
	return selected, nil
}

func createWriteReadSmall(_ context.Context, env *harness.Env) error {
	path := filepath.Join(env.MountDir, "alpha.txt")
	want := []byte("hello from wsfs\n")

	if err := os.WriteFile(path, want, 0o644); err != nil {
		return err
	}
	got, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if !bytes.Equal(got, want) {
		return fmt.Errorf("mount content mismatch: got %q want %q", got, want)
	}

	backend, err := os.ReadFile(filepath.Join(env.BackendDir, "alpha.txt"))
	if err != nil {
		return err
	}
	if !bytes.Equal(backend, want) {
		return fmt.Errorf("backend content mismatch: got %q want %q", backend, want)
	}
	return nil
}

func overwriteExistingFile(_ context.Context, env *harness.Env) error {
	backendPath := filepath.Join(env.BackendDir, "payload.txt")
	if err := os.WriteFile(backendPath, []byte("old-data"), 0o644); err != nil {
		return err
	}

	mountPath := filepath.Join(env.MountDir, "payload.txt")
	want := []byte("new-data")
	if err := os.WriteFile(mountPath, want, 0o644); err != nil {
		return err
	}

	got, err := os.ReadFile(backendPath)
	if err != nil {
		return err
	}
	if !bytes.Equal(got, want) {
		return fmt.Errorf("backend content mismatch: got %q want %q", got, want)
	}
	return nil
}

func appendViaStdWrite(_ context.Context, env *harness.Env) error {
	path := filepath.Join(env.MountDir, "append.txt")
	if err := os.WriteFile(path, []byte("head"), 0o644); err != nil {
		return err
	}

	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		return err
	}
	if _, err := f.Write([]byte("-tail")); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}

	want := []byte("head-tail")
	got, err := os.ReadFile(filepath.Join(env.BackendDir, "append.txt"))
	if err != nil {
		return err
	}
	if !bytes.Equal(got, want) {
		return fmt.Errorf("append mismatch: got %q want %q", got, want)
	}
	return nil
}

func truncateShrink(_ context.Context, env *harness.Env) error {
	path := filepath.Join(env.MountDir, "shrink.txt")
	if err := os.WriteFile(path, []byte("abcdefghij"), 0o644); err != nil {
		return err
	}
	if err := os.Truncate(path, 4); err != nil {
		return err
	}

	got, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if string(got) != "abcd" {
		return fmt.Errorf("truncate shrink mismatch: got %q", got)
	}
	return nil
}

func truncateExpand(_ context.Context, env *harness.Env) error {
	path := filepath.Join(env.MountDir, "expand.bin")
	if err := os.WriteFile(path, []byte("abc"), 0o644); err != nil {
		return err
	}
	if err := os.Truncate(path, 8); err != nil {
		return err
	}

	got, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	want := []byte{'a', 'b', 'c', 0, 0, 0, 0, 0}
	if !bytes.Equal(got, want) {
		return fmt.Errorf("truncate expand mismatch: got %v want %v", got, want)
	}
	return nil
}

func getattrAfterWrite(_ context.Context, env *harness.Env) error {
	path := filepath.Join(env.MountDir, "stat-after-write.bin")
	payload := []byte("1234567890abcdef")
	if err := os.WriteFile(path, payload, 0o644); err != nil {
		return err
	}

	size, err := harness.StatSize(path)
	if err != nil {
		return err
	}
	if size != int64(len(payload)) {
		return fmt.Errorf("mount stat size mismatch: got %d want %d", size, len(payload))
	}

	backendSize, err := harness.StatSize(filepath.Join(env.BackendDir, "stat-after-write.bin"))
	if err != nil {
		return err
	}
	if backendSize != int64(len(payload)) {
		return fmt.Errorf("backend stat size mismatch: got %d want %d", backendSize, len(payload))
	}
	return nil
}

func getattrAfterRename(_ context.Context, env *harness.Env) error {
	oldPath := filepath.Join(env.MountDir, "old-name.txt")
	newPath := filepath.Join(env.MountDir, "new-name.txt")
	if err := os.WriteFile(oldPath, []byte("rename-check"), 0o644); err != nil {
		return err
	}
	if err := os.Rename(oldPath, newPath); err != nil {
		return err
	}

	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		return fmt.Errorf("old path stat mismatch after rename: %v", err)
	}
	size, err := harness.StatSize(newPath)
	if err != nil {
		return err
	}
	if size != int64(len("rename-check")) {
		return fmt.Errorf("new path stat size mismatch: got %d", size)
	}
	return nil
}

func readAtOffsets(_ context.Context, env *harness.Env) error {
	mountPath := filepath.Join(env.MountDir, "offsets.txt")
	const content = "0123456789abcdefghijklmnopqrstuvwxyz"
	tests := []struct {
		off  int64
		size int
		want string
	}{
		{off: 0, size: 5, want: "01234"},
		{off: 10, size: 6, want: "abcdef"},
		{off: int64(len(content) - 3), size: 8, want: "xyz"},
	}
	for _, tc := range tests {
		buf := make([]byte, tc.size)
		f, err := os.Open(mountPath)
		if err != nil {
			return err
		}
		n, err := f.ReadAt(buf, tc.off)
		_ = f.Close()
		if err != nil && !strings.Contains(err.Error(), "EOF") {
			return err
		}
		if got := string(buf[:n]); got != tc.want {
			return fmt.Errorf("readat off=%d got %q want %q", tc.off, got, tc.want)
		}
	}
	return nil
}

func prepareReadAtOffsets(env *harness.Env) error {
	data := []byte("0123456789abcdefghijklmnopqrstuvwxyz")
	return os.WriteFile(filepath.Join(env.BackendDir, "offsets.txt"), data, 0o644)
}

func prepareReadLargeFile(env *harness.Env) error {
	payload := make([]byte, 20000)
	for i := range payload {
		payload[i] = byte((i*17 + 9) % 251)
	}
	return os.WriteFile(filepath.Join(env.BackendDir, "large-read.bin"), payload, 0o644)
}

func writeAtOffsets(_ context.Context, env *harness.Env) error {
	path := filepath.Join(env.MountDir, "patch.txt")
	if err := os.WriteFile(path, []byte("0123456789"), 0o644); err != nil {
		return err
	}

	f, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	if _, err := f.WriteAt([]byte("AB"), 2); err != nil {
		_ = f.Close()
		return err
	}
	if _, err := f.WriteAt([]byte("XYZ"), 7); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}

	got, err := os.ReadFile(filepath.Join(env.BackendDir, "patch.txt"))
	if err != nil {
		return err
	}
	if string(got) != "01AB456XYZ" {
		return fmt.Errorf("writeat mismatch: got %q", got)
	}
	return nil
}

func readdirRoot(_ context.Context, env *harness.Env) error {
	got, err := harness.SortedNames(env.MountDir)
	if err != nil {
		return err
	}
	want := []string{"a.txt", "b.txt", "dir"}
	if !reflect.DeepEqual(got, want) {
		return fmt.Errorf("readdir mismatch: got %v want %v", got, want)
	}
	return nil
}

func prepareReaddirRoot(env *harness.Env) error {
	for _, name := range []string{"a.txt", "b.txt"} {
		if err := os.WriteFile(filepath.Join(env.BackendDir, name), []byte(name), 0o644); err != nil {
			return err
		}
	}
	return os.Mkdir(filepath.Join(env.BackendDir, "dir"), 0o755)
}

func readLargeFileCrossMessageBoundary(_ context.Context, env *harness.Env) error {
	want, err := os.ReadFile(filepath.Join(env.BackendDir, "large-read.bin"))
	if err != nil {
		return err
	}
	got, err := os.ReadFile(filepath.Join(env.MountDir, "large-read.bin"))
	if err != nil {
		return err
	}
	if !bytes.Equal(got, want) {
		return fmt.Errorf("large read mismatch: got %d bytes want %d bytes", len(got), len(want))
	}
	return nil
}

func createManyEntries(_ context.Context, env *harness.Env) error {
	const count = 96
	for i := range count {
		name := fmt.Sprintf("file-%03d.txt", i)
		if err := os.WriteFile(filepath.Join(env.MountDir, name), []byte(name), 0o644); err != nil {
			return err
		}
	}

	got, err := harness.SortedNames(env.MountDir)
	if err != nil {
		return err
	}
	if len(got) != count {
		return fmt.Errorf("entry count mismatch: got %d want %d", len(got), count)
	}
	return nil
}

func mkdirThenReaddir(_ context.Context, env *harness.Env) error {
	dir := filepath.Join(env.MountDir, "newdir")
	if err := os.Mkdir(dir, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "child.txt"), []byte("child"), 0o644); err != nil {
		return err
	}

	got, err := harness.SortedNames(env.MountDir)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(got, []string{"newdir"}) {
		return fmt.Errorf("root dir mismatch after mkdir: %v", got)
	}

	childNames, err := harness.SortedNames(filepath.Join(env.BackendDir, "newdir"))
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(childNames, []string{"child.txt"}) {
		return fmt.Errorf("backend child dir mismatch: %v", childNames)
	}
	return nil
}

func renameFileCrossDir(_ context.Context, env *harness.Env) error {
	srcDir := filepath.Join(env.MountDir, "src")
	dstDir := filepath.Join(env.MountDir, "dst")
	for _, dir := range []string{srcDir, dstDir} {
		if err := os.Mkdir(dir, 0o755); err != nil {
			return err
		}
	}

	oldPath := filepath.Join(srcDir, "payload.txt")
	newPath := filepath.Join(dstDir, "renamed.txt")
	if err := os.WriteFile(oldPath, []byte("move-me"), 0o644); err != nil {
		return err
	}
	if err := os.Rename(oldPath, newPath); err != nil {
		return err
	}

	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		return fmt.Errorf("old path still exists or unexpected err: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(env.BackendDir, "dst", "renamed.txt"))
	if err != nil {
		return err
	}
	if string(got) != "move-me" {
		return fmt.Errorf("renamed content mismatch: got %q", got)
	}
	return nil
}

func renameThenReaddirOldParent(_ context.Context, env *harness.Env) error {
	oldDir := filepath.Join(env.MountDir, "old-parent")
	newDir := filepath.Join(env.MountDir, "new-parent")
	if err := os.Mkdir(oldDir, 0o755); err != nil {
		return err
	}
	if err := os.Mkdir(newDir, 0o755); err != nil {
		return err
	}

	oldPath := filepath.Join(oldDir, "move.txt")
	newPath := filepath.Join(newDir, "move.txt")
	if err := os.WriteFile(oldPath, []byte("move"), 0o644); err != nil {
		return err
	}
	if err := os.Rename(oldPath, newPath); err != nil {
		return err
	}

	oldNames, err := harness.SortedNames(oldDir)
	if err != nil {
		return err
	}
	if len(oldNames) != 0 {
		return fmt.Errorf("old parent not empty after rename: %v", oldNames)
	}
	newNames, err := harness.SortedNames(newDir)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(newNames, []string{"move.txt"}) {
		return fmt.Errorf("new parent entries mismatch: %v", newNames)
	}
	return nil
}

func removeThenReaddir(_ context.Context, env *harness.Env) error {
	for _, name := range []string{"gone-a.txt", "gone-b.txt"} {
		if err := os.WriteFile(filepath.Join(env.MountDir, name), []byte(name), 0o644); err != nil {
			return err
		}
	}
	for _, name := range []string{"gone-a.txt", "gone-b.txt"} {
		if err := os.Remove(filepath.Join(env.MountDir, name)); err != nil {
			return err
		}
	}

	got, err := harness.SortedNames(env.MountDir)
	if err != nil {
		return err
	}
	if len(got) != 0 {
		return fmt.Errorf("directory not empty after remove: %v", got)
	}
	return nil
}

func createAndReadSymlink(_ context.Context, env *harness.Env) error {
	if runtime.GOOS == "windows" {
		return harness.Skip("skip symlinks on Windows")
	}

	targetName := "target.txt"
	targetPath := filepath.Join(env.MountDir, targetName)
	if err := os.WriteFile(targetPath, []byte("symlink-target"), 0o644); err != nil {
		return err
	}

	linkPath := filepath.Join(env.MountDir, "link.txt")
	if err := os.Symlink(targetPath, linkPath); err != nil {
		return err
	}

	got, err := os.Readlink(linkPath)
	if err != nil {
		return err
	}
	wantMountTarget := filepath.Join(env.MountDir, targetName)
	if got != wantMountTarget {
		return fmt.Errorf("readlink mismatch: got %q want %q", got, wantMountTarget)
	}

	backendTarget, err := os.Readlink(filepath.Join(env.BackendDir, "link.txt"))
	if err != nil {
		return err
	}
	wantBackendPrefix := filepath.Join(env.BackendDir, targetName)
	if backendTarget != wantBackendPrefix {
		return fmt.Errorf("backend symlink mismatch: got %q want %q", backendTarget, wantBackendPrefix)
	}
	return nil
}

func readFileViaSymlink(_ context.Context, env *harness.Env) error {
	if runtime.GOOS == "windows" {
		return harness.Skip("skip symlinks on Windows")
	}

	targetPath := filepath.Join(env.MountDir, "target-read.txt")
	if err := os.WriteFile(targetPath, []byte("through-link"), 0o644); err != nil {
		return err
	}

	linkPath := filepath.Join(env.MountDir, "link-read.txt")
	if err := os.Symlink(targetPath, linkPath); err != nil {
		return err
	}

	got, err := os.ReadFile(linkPath)
	if err != nil {
		return err
	}
	if string(got) != "through-link" {
		return fmt.Errorf("symlink read mismatch: got %q", got)
	}
	return nil
}

func manySmallRandomWrites(_ context.Context, env *harness.Env) error {
	const (
		fileSize   = 8192
		writeCount = 128
	)

	path := filepath.Join(env.MountDir, "random-writes.bin")
	expected := make([]byte, fileSize)
	if err := os.WriteFile(path, expected, 0o644); err != nil {
		return err
	}

	f, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		return err
	}

	rng := rand.New(rand.NewSource(1))
	for range writeCount {
		size := rng.Intn(32) + 1
		off := rng.Intn(fileSize - size + 1)
		chunk := make([]byte, size)
		for i := range chunk {
			chunk[i] = byte(rng.Intn(251))
		}
		copy(expected[off:off+size], chunk)
		if _, err := f.WriteAt(chunk, int64(off)); err != nil {
			_ = f.Close()
			return err
		}
	}
	if err := f.Close(); err != nil {
		return err
	}

	gotBackend, err := os.ReadFile(filepath.Join(env.BackendDir, "random-writes.bin"))
	if err != nil {
		return err
	}
	if !bytes.Equal(gotBackend, expected) {
		return fmt.Errorf("backend random write mismatch")
	}
	gotMount, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if !bytes.Equal(gotMount, expected) {
		return fmt.Errorf("mount random write mismatch")
	}
	return nil
}

func writeLargeFileCrossMessageBoundary(_ context.Context, env *harness.Env) error {
	path := filepath.Join(env.MountDir, "large.bin")
	payload := make([]byte, 20000)
	for i := range payload {
		payload[i] = byte((i * 31) % 251)
	}
	if err := os.WriteFile(path, payload, 0o644); err != nil {
		return err
	}

	got, err := os.ReadFile(filepath.Join(env.BackendDir, "large.bin"))
	if err != nil {
		return err
	}
	if !bytes.Equal(got, payload) {
		return fmt.Errorf("large payload mismatch: got %d bytes want %d bytes", len(got), len(payload))
	}

	readBack, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if !bytes.Equal(readBack, payload) {
		return fmt.Errorf("large mount readback mismatch")
	}
	return nil
}

func Names() []string {
	all := All()
	names := make([]string, 0, len(all))
	for _, c := range all {
		names = append(names, c.Name())
	}
	sort.Strings(names)
	return names
}
