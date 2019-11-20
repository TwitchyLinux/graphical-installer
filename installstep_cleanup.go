package main

type CleanupStep struct {
}

func (s *CleanupStep) Run(updateChan chan progressUpdate, installState *installState) error {
	if err := runCmd(updateChan, "[UNMOUNT]: ", "umount", "/tmp/install_mounts/boot"); err != nil {
		return err
	}
	if err := runCmd(updateChan, "[UNMOUNT]: ", "umount", "/tmp/install_mounts/root"); err != nil {
		return err
	}
	return nil
}

func (s *CleanupStep) Name() string {
	return "Cleanup"
}
