package main

import (
	"bytes"
	"os/exec"
	"strconv"
	"time"
)

const (
	blockSize = 512

	bootPartSizeMB = 256
	bootPartBlocks = bootPartSizeMB * 1024 * 1024 / blockSize

	metadataPartSizeMB = 64
	metadataPartBlocks = metadataPartSizeMB * 1024 * 1024 / blockSize

	unallocBlocks = 128
)

type PartitionStep struct {
}

func (s *PartitionStep) Run(updateChan chan progressUpdate, installState *installState) error {
	progressInfo(updateChan, "Partitioning %q\n", installState.InstallDevice.Path)
	progressInfo(updateChan, "Device has a capacity of %s\n", byteCountDecimal(int64(installState.InstallDevice.NumBlocks*blockSize)))
	progressInfo(updateChan, "\n  New partition table:\n")

	mainPartBlocks := installState.InstallDevice.NumBlocks - bootPartBlocks - metadataPartBlocks - unallocBlocks
	mainPartMB := mainPartBlocks * blockSize / 1024 / 1024

	progressInfo(updateChan, "    [EXT4]  Boot partition (%s)\n", byteCountDecimal(bootPartSizeMB*1000*1000))
	progressInfo(updateChan, "    [LUKS]  Encrypted root partition (%s)\n", byteCountDecimal(int64(mainPartBlocks*blockSize)))
	progressInfo(updateChan, "    [EXT4]  Encrypted TwitchyLinux metadata partition (%s)\n", byteCountDecimal(metadataPartSizeMB*1000*1000))

	cmd := exec.Command("parted", "--script", installState.InstallDevice.Path, "mklabel", "msdos",
		"mkpart", "p", "ext4", "1", strconv.Itoa(bootPartSizeMB),
		"mkpart", "p", strconv.Itoa(1+bootPartSizeMB), strconv.Itoa(1+bootPartSizeMB+mainPartMB),
		"mkpart", "p", strconv.Itoa(1+bootPartSizeMB+mainPartMB), strconv.Itoa(1+bootPartSizeMB+mainPartMB+metadataPartSizeMB),
		"set", "1", "boot", "on")

	progressInfo(updateChan, "\n  Parted invocation: %v\n", cmd.Args)

	out, err := cmd.CombinedOutput()
	progressInfo(updateChan, "  Output: %q\n", string(out))
	if err != nil {
		return err
	}
	time.Sleep(time.Second)

	cmd = exec.Command("partprobe", installState.InstallDevice.Path)
	progressInfo(updateChan, "\n  Probing: %v\n", installState.InstallDevice.Path)
	out, err = cmd.CombinedOutput()
	progressInfo(updateChan, "  Output: %q\n", string(out))
	if err != nil {
		return err
	}
	time.Sleep(3 * time.Second)

	cmd = exec.Command("mkfs.ext4", "-qF", installState.InstallDevice.pathForPartition(1))
	progressInfo(updateChan, "\n  Creating ext4 filesystem on %v\n", installState.InstallDevice.pathForPartition(1))
	out, err = cmd.CombinedOutput()
	progressInfo(updateChan, "  Output: %q\n", string(out))
	if err != nil {
		return err
	}
	time.Sleep(1 * time.Second)

	cmd = exec.Command("cryptsetup", "luksFormat", "--type", "luks2", installState.InstallDevice.pathForPartition(2), "--key-file", "-",
		"--hash", "sha256", "--cipher", "aes-xts-plain64", "--key-size", "512", "--iter-time", "2600", "--use-random")
	progressInfo(updateChan, "\n  Creating encrypted filesystem on %v\n", installState.InstallDevice.pathForPartition(2))
	progressInfo(updateChan, "  Invocation: %v\n", cmd.Args)
	cmd.Stdin = bytes.NewReader([]byte(installState.Pw))
	out, err = cmd.CombinedOutput()
	progressInfo(updateChan, "  Output: %q\n", string(out))
	if err != nil {
		return err
	}
	time.Sleep(1 * time.Second)

	progressInfo(updateChan, "\n  Unlocking root filesystem\n")
	cmd = exec.Command("cryptsetup", "luksOpen", "--key-file", "-", installState.InstallDevice.pathForPartition(2), "cryptroot")
	progressInfo(updateChan, "  Invocation: %v\n", cmd.Args)
	cmd.Stdin = bytes.NewReader([]byte(installState.Pw))
	out, err = cmd.CombinedOutput()
	progressInfo(updateChan, "  Output: %q\n", string(out))
	if err != nil {
		return err
	}
	time.Sleep(1 * time.Second)

	if installState.Scrub {
		if err := s.scrubEncrypted(updateChan, installState); err != nil {
			return err
		}
	}

	cmd = exec.Command("mkfs.ext4", "-qF", "/dev/mapper/cryptroot")
	progressInfo(updateChan, "\n  Creating ext4 filesystem on %v\n", "/dev/mapper/cryptroot")
	out, err = cmd.CombinedOutput()
	progressInfo(updateChan, "  Output: %q\n", string(out))
	if err != nil {
		return err
	}
	time.Sleep(1 * time.Second)

	cmd = exec.Command("mkfs.ext4", "-qF", installState.InstallDevice.pathForPartition(3))
	progressInfo(updateChan, "\n  Creating ext4 filesystem on %v\n", installState.InstallDevice.pathForPartition(3))
	out, err = cmd.CombinedOutput()
	progressInfo(updateChan, "  Output: %q\n", string(out))
	if err != nil {
		return err
	}
	time.Sleep(1 * time.Second)

	return nil
}

func (s *PartitionStep) scrubEncrypted(updateChan chan progressUpdate, installState *installState) error {

	progressInfo(updateChan, "\n  Scrubbing encrypted partition:\n")
	e := exec.Command("dd", "if=/dev/zero", "of=/dev/mapper/cryptroot", "bs=1M", "status=progress")
	progressInfo(updateChan, "  Invocation: %v\n", e.Args)

	e.Stdout = &cmdInteractiveWriter{
		updateChan: updateChan,
		logPrefix:  "  ",
		IsProgress: true,
	}
	e.Stderr = e.Stdout

	if err := e.Start(); err != nil {
		return err
	}

	e.Wait() // Will error when we exhaust the space on the device (intended).
	return nil
}

func (s *PartitionStep) Name() string {
	return "Format disk"
}
