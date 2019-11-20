package main

import (
	"fmt"
	"os"
	"path"
	"syscall"
	"time"
)

const (
	ext4Opts  = "journal_checksum,journal_ioprio=0,data=writeback,barrier=0,errors=remount-ro"
	ext4Flags = syscall.MS_DIRSYNC | syscall.MS_NOSUID | syscall.MS_NOATIME
)

var rootFSCopyOps = []copyOp{
	{
		From: "/bin",
		To:   "/bin",
	},
	{
		From: "/etc",
		To:   "/etc",
	},
	{
		From: "/home",
		To:   "/home",
	},
	{
		From: "/lib",
		To:   "/lib",
	},
	{
		From: "/lib32",
		To:   "/lib32",
	},
	{
		From: "/lib64",
		To:   "/lib64",
	},
	{
		From: "/libx32",
		To:   "/libx32",
	},
	{
		From: "/opt",
		To:   "/opt",
	},
	{
		From: "/root",
		To:   "/root",
	},
	{
		From: "/sbin",
		To:   "/sbin",
	},
	{
		From: "/srv",
		To:   "/srv",
	},
	{
		From: "/usr",
		To:   "/usr",
	},
	{
		From: "/var",
		To:   "/var",
	},
	{
		From: "/deb-pkgs",
		To:   "/deb-pkgs",
	},
}

type copyOp struct {
	From, To string
}

type CopyStep struct {
}

func (s *CopyStep) Run(updateChan chan progressUpdate, installState *installState) error {
	if err := os.Mkdir("/tmp/install_mounts", 0755); err != nil && !os.IsExist(err) {
		return err
	}
	if err := os.Mkdir("/tmp/install_mounts/root", 0755); err != nil && !os.IsExist(err) {
		return err
	}
	if err := os.Mkdir("/tmp/install_mounts/boot", 0755); err != nil && !os.IsExist(err) {
		return err
	}
	time.Sleep(1 * time.Second)

	progressInfo(updateChan, "Mounting %s -> /tmp/install_mounts/boot\n    Opts: %q\n", installState.InstallDevice.Path+"1", ext4Opts)
	if err := syscall.Mount(installState.InstallDevice.Path+"1", "/tmp/install_mounts/boot", "ext4", ext4Flags, ext4Opts); err != nil {
		return fmt.Errorf("failed to mount dev filesystem: %v", err)
	}
	progressInfo(updateChan, "Mounted boot fs.\n")
	time.Sleep(3 * time.Second)

	if err := runCmd(updateChan, "[BOOT]: Install ", "cp", "-a", "--no-target-directory", "/boot/boot", "/tmp/install_mounts/boot"); err != nil {
		return err
	}
	time.Sleep(1 * time.Second)

	progressInfo(updateChan, "\n  Mounting %s -> /tmp/install_mounts/root\n", "/dev/mapper/cryptroot")
	if err := syscall.Mount("/dev/mapper/cryptroot", "/tmp/install_mounts/root", "ext4", ext4Flags, ext4Opts); err != nil {
		return fmt.Errorf("failed to mount root filesystem: %v", err)
	}
	progressInfo(updateChan, "Mounted root fs.\n\n")
	time.Sleep(2 * time.Second)

	if err := s.CreateSysPaths(updateChan, installState); err != nil {
		return err
	}

	for _, op := range rootFSCopyOps {
		if err := runCmd(updateChan, "[ROOT]: Install ", "cp", "-a", op.From, path.Join("/tmp/install_mounts/root", op.To)); err != nil {
			return err
		}
	}

	return nil
}

func (s *CopyStep) CreateSysPaths(updateChan chan progressUpdate, installState *installState) error {
	if err := runCmd(updateChan, "[ROOT]: Create ", "mkdir", "-p", "/tmp/install_mounts/root/dev", "/tmp/install_mounts/root/proc"); err != nil {
		return err
	}
	if err := runCmd(updateChan, "[ROOT]: Create ", "mkdir", "-p", "/tmp/install_mounts/root/sys", "/tmp/install_mounts/root/run"); err != nil {
		return err
	}
	if err := runCmd(updateChan, "[ROOT]: Create ", "mkdir", "-p", "/tmp/install_mounts/root/media", "/tmp/install_mounts/root/mnt"); err != nil {
		return err
	}
	if err := runCmd(updateChan, "[ROOT]: Create ", "mkdir", "-p", "/tmp/install_mounts/root/tmp", "/tmp/install_mounts/root/boot"); err != nil {
		return err
	}
	if err := runCmd(updateChan, "[ROOT]: Mknod ", "mknod", "-m", "600", "/tmp/install_mounts/root/dev/console", "c", "5", "1"); err != nil {
		return err
	}
	if err := runCmd(updateChan, "[ROOT]: Mknod ", "mknod", "-m", "600", "/tmp/install_mounts/root/dev/null", "c", "1", "3"); err != nil {
		return err
	}

	time.Sleep(1 * time.Second)
	return nil
}

func (s *CopyStep) Name() string {
	return "Copy files"
}
