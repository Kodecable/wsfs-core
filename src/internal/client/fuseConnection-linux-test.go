//go:build linux

package client

import "testing"

func TestFuseConnectionIDFromMountInfo(t *testing.T) {
	mountInfo := "42 1 0:77 / /mnt/wsfs\\040test rw,nosuid - fuse.wsfs wsfs rw\n" +
		"43 42 0:78 / /mnt/wsfs\\040test/child rw,nosuid - fuse.wsfs wsfs rw\n"

	id, err := fuseConnectionIDFromMountInfo(mountInfo, "/mnt/wsfs test")
	if err != nil {
		t.Fatalf("fuseConnectionIDFromMountInfo: %v", err)
	}
	if id != 77 {
		t.Fatalf("connection ID = %d, want 77", id)
	}
}

func TestFuseConnectionIDFromMountInfoRejectsNonFuseMount(t *testing.T) {
	mountInfo := "42 1 0:77 / /mnt/wsfs rw,nosuid - ext4 disk rw\n"
	if _, err := fuseConnectionIDFromMountInfo(mountInfo, "/mnt/wsfs"); err == nil {
		t.Fatal("expected non-FUSE mount to be rejected")
	}
}

func TestFuseConnectionIDFromMountInfoRejectsFusectl(t *testing.T) {
	mountInfo := "42 1 0:77 / /mnt/wsfs rw,nosuid - fusectl fusectl rw\n"
	if _, err := fuseConnectionIDFromMountInfo(mountInfo, "/mnt/wsfs"); err == nil {
		t.Fatal("expected fusectl mount to be rejected")
	}
}
