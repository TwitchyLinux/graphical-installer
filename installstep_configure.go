package main

import (
	"bytes"
	"errors"
	"io/ioutil"
	"os/exec"
	"path"
	"strings"
	"time"
)

type ConfigureStep struct {
}

func getUUID(updateChan chan progressUpdate, dev string) (string, error) {
	cmd := exec.Command("lsblk", "--nodeps", "-nr", "-o", "UUID", dev)
	out, err := cmd.Output()
	if err != nil {
		progressInfo(updateChan, "Failing invocation: %q\n", cmd.Args)
		return "", err
	}
	return strings.Trim(string(out), " \t\r\n"), nil
}

func kernelInfo(updateChan chan progressUpdate) (string, string, error) {
	ff, err := ioutil.ReadDir("/tmp/install_mounts/boot")
	if err != nil {
		return "", "", err
	}
	for _, f := range ff {
		progressInfo(updateChan, "Scan file: %q\n", f.Name())
		if !f.IsDir() && strings.HasPrefix(f.Name(), "vmlinuz-") {
			return f.Name(), strings.Split(f.Name(), "-")[1], nil
		}
	}
	return "", "", errors.New("could not determine current kernel")
}

func (s *ConfigureStep) Run(updateChan chan progressUpdate, installState *installState) error {
	bootUUID, err := getUUID(updateChan, installState.InstallDevice.pathForPartition(1))
	if err != nil {
		return err
	}
	progressInfo(updateChan, "Boot UUID: %q\n", bootUUID)
	encUUID, err := getUUID(updateChan, installState.InstallDevice.pathForPartition(2))
	if err != nil {
		return err
	}
	progressInfo(updateChan, "LUKS UUID: %q\n", encUUID)

	// Write out /etc/{fstab,cryptab}
	fstab := strings.Replace(fstabData, "FSTAB_DEV", encUUID, -1)
	fstab = strings.Replace(fstab, "BOOT_DEV", bootUUID, -1)
	if err := ioutil.WriteFile(path.Join("/tmp/install_mounts/root", "etc/fstab"), []byte(fstab), 0550); err != nil {
		return err
	}
	progressInfo(updateChan, "/etc/fstab written to %q\n", path.Join("/tmp/install_mounts/root", "etc/fstab"))
	time.Sleep(time.Second)

	crypttab := "cryptroot UUID=" + encUUID + " none luks,discard\n"
	if err := ioutil.WriteFile(path.Join("/tmp/install_mounts/root", "etc/crypttab"), []byte(crypttab), 0550); err != nil {
		return err
	}
	progressInfo(updateChan, "/etc/crypttab written to %q\n", path.Join("/tmp/install_mounts/root", "etc/crypttab"))

	// Write out /boot/grub/grub.cfg
	kernPath, kernVersion, err := kernelInfo(updateChan)
	if err != nil {
		return err
	}
	progressInfo(updateChan, "Will boot kernel image at %q (%s)\n", kernPath, kernVersion)
	grubCfg := strings.Replace(grubData, "KERN_IMG_FILENAME", kernPath, -1)
	grubCfg = strings.Replace(grubCfg, "K_VERS", kernVersion, -1)
	grubCfg = strings.Replace(grubCfg, "MAIN_PART_UUID", encUUID, -1)
	grubCfg = strings.Replace(grubCfg, "BOOT_PART_UUID", bootUUID, -1)
	if err := ioutil.WriteFile(path.Join("/tmp/install_mounts/boot", "grub/grub.cfg"), []byte(grubCfg), 0550); err != nil {
		return err
	}
	progressInfo(updateChan, "grub.cfg written to %q\n", path.Join("/tmp/install_mounts/boot", "grub/grub.cfg"))
	time.Sleep(time.Second)

	// Run grub-install
	if err := ioutil.WriteFile("/tmp/device.map", []byte("(hd0) "+installState.InstallDevice.Path), 0550); err != nil {
		return err
	}
	if err := runCmd(updateChan, "[GRUB-INSTALL]: ", "grub-install", "--no-floppy", "--grub-mkdevicemap=/tmp/device.map",
		"--boot-directory=/tmp/install_mounts/boot", "--root-directory=/tmp/install_mounts/root",
		installState.InstallDevice.Path); err != nil {
		return err
	}
	progressInfo(updateChan, "Finished installing bootloader (grub2).\n\n")
	if err := ioutil.WriteFile(path.Join("/tmp/install_mounts/root", "etc/hostname"), []byte(installState.Host+"\n"), 0012); err != nil {
		return err
	}

	if err := s.runChrootSteps(updateChan, installState); err != nil {
		return err
	}

	if err := ioutil.WriteFile(path.Join("/tmp/install_mounts/root", "etc/timezone"), []byte(installState.Tz+"\n"), 0012); err != nil {
		return err
	}
	progressInfo(updateChan, "%q written to %q\n", installState.Tz, path.Join("/tmp/install_mounts/root", "etc/timezone"))
	if err := runCmd(updateChan, "[TIMEZONE]: Install ", "cp", "/usr/share/zoneinfo/"+installState.Tz, "/tmp/install_mounts/root/etc/localtime"); err != nil {
		return err
	}
	time.Sleep(time.Second)
	return nil
}

func (s *ConfigureStep) runChrootSteps(updateChan chan progressUpdate, installState *installState) error {
	// Setup a chroot for the update-initramfs command.
	if err := runCmd(updateChan, "[CHROOT-SETUP]: ", "mount", "-v", "--bind", "/dev", path.Join("/tmp/install_mounts/root", "dev")); err != nil {
		return err
	}
	defer runCmd(updateChan, "[CHROOT-UNSETUP]: ", "umount", path.Join("/tmp/install_mounts/root", "dev"))
	if err := runCmd(updateChan, "[CHROOT-SETUP]: ", "mount", "-vt", "devpts", "devpts", path.Join("/tmp/install_mounts/root", "dev/pts"), "-o", "gid=5,mode=620"); err != nil {
		return err
	}
	defer runCmd(updateChan, "[CHROOT-UNSETUP]: ", "umount", path.Join("/tmp/install_mounts/root", "dev/pts"))
	if err := runCmd(updateChan, "[CHROOT-SETUP]: ", "mount", "-vt", "proc", "proc", path.Join("/tmp/install_mounts/root", "proc")); err != nil {
		return err
	}
	defer runCmd(updateChan, "[CHROOT-UNSETUP]: ", "umount", path.Join("/tmp/install_mounts/root", "proc"))
	if err := runCmd(updateChan, "[CHROOT-SETUP]: ", "mount", "-vt", "sysfs", "sysfs", path.Join("/tmp/install_mounts/root", "sys")); err != nil {
		return err
	}
	defer runCmd(updateChan, "[CHROOT-UNSETUP]: ", "umount", path.Join("/tmp/install_mounts/root", "sys"))
	if err := runCmd(updateChan, "[CHROOT-SETUP]: ", "mount", "-vt", "tmpfs", "tmpfs", path.Join("/tmp/install_mounts/root", "run")); err != nil {
		return err
	}
	defer runCmd(updateChan, "[CHROOT-UNSETUP]: ", "umount", path.Join("/tmp/install_mounts/root", "run"))
	if err := runCmd(updateChan, "[CHROOT-SETUP]: ", "mount", "-v", "--bind", "/tmp/install_mounts/boot", path.Join("/tmp/install_mounts/root", "boot")); err != nil {
		return err
	}
	defer runCmd(updateChan, "[CHROOT-UNSETUP]: ", "umount", path.Join("/tmp/install_mounts/root", "boot"))

	if err := runCmdInteractive(updateChan, "[INITRAMFS]: ", "chroot", "/tmp/install_mounts/root", "dpkg-reconfigure", "--frontend=noninteractive", "cryptsetup-initramfs"); err != nil {
		return err
	}

	if err := runCmdInteractive(updateChan, "[INITRAMFS]: ", "chroot", "/tmp/install_mounts/root", "update-initramfs", "-u", "-v"); err != nil {
		return err
	}

	progressInfo(updateChan, "\n  Updating user account setup.\n")
	cmd := exec.Command("chroot", "/tmp/install_mounts/root", "chpasswd", "-c", "SHA512")
	cmd.Stdin = bytes.NewBufferString("twl:" + installState.Pw + "\n")
	out, err := cmd.CombinedOutput()
	if err != nil {
		progressInfo(updateChan, "  Output: %q\n", out)
		return err
	}
	time.Sleep(time.Second)
	cmd = exec.Command("chroot", "/tmp/install_mounts/root", "chpasswd", "-c", "SHA512")
	cmd.Stdin = bytes.NewBufferString("root:" + installState.Pw + "\n")
	out, err = cmd.CombinedOutput()
	if err != nil {
		progressInfo(updateChan, "  Output: %q\n", out)
		return err
	}
	time.Sleep(time.Second)

	if installState.User != "twl" {
		progressInfo(updateChan, "Renaming %q -> %q.\n", "twl", installState.User)
		if err := runCmdInteractive(updateChan, "  [SETUP-USER]: ", "chroot", "/tmp/install_mounts/root", "usermod", "--login", installState.User, "--move-home",
			"--home", path.Join("/home", installState.User), "twl"); err != nil {
			return err
		}
		if err := runCmdInteractive(updateChan, "  [SETUP-USERGROUP]: ", "chroot", "/tmp/install_mounts/root", "groupmod", "--new-name", installState.User, "twl"); err != nil {
			return err
		}
	}

	if installState.Autologin {
		progressInfo(updateChan, "\n  Switching getty@.service with autologin@.service.\n")
		if err := runCmdInteractive(updateChan, "  [SETUP-AUTOLOGIN]: ", "chroot", "/tmp/install_mounts/root", "cp", "/usr/share/twlinst/autologin-template", "/lib/systemd/system/autologin@.service"); err != nil {
			return err
		}
		if err := runCmdInteractive(updateChan, "  [SETUP-AUTOLOGIN]: ", "chroot", "/tmp/install_mounts/root", "ln", "-s", "../autologin@.service", "/lib/systemd/system/getty.target.wants/autologin@tty1.service"); err != nil {
			return err
		}
		if err := runCmdInteractive(updateChan, "  [SETUP-AUTOLOGIN]: ", "chroot", "/tmp/install_mounts/root", "sed", "-i", "s/USERNAME/"+installState.User+"/g", "/lib/systemd/system/autologin@.service"); err != nil {
			return err
		}
	}

	for _, pkg := range installState.OptionalPkgs {
		progressInfo(updateChan, "\n")
		if err := runCmdInteractive(updateChan, "  [INSTALL]: ", "chroot", "/tmp/install_mounts/root", "bash", "-c", "dpkg -i /deb-pkgs/"+pkg+"/*.deb"); err != nil {
			return err
		}
	}
	return nil
}

func (s *ConfigureStep) Name() string {
	return "Configure system"
}
