package main

const grubData = `# Set menu colors
set menu_color_normal=white/black
set menu_color_highlight=black/white
#loadfont ($root)/boot/grub/fonts/unicode.pf2
#set theme=($root)/boot/grub/theme.txt

# Set menu display time
set timeout=7

# Setup graphics
function load_video {
  insmod vbe
  insmod vga
  insmod video_bochs
  insmod video_cirrus
}

#set gfxmode=1024x768x32,1024x768x16,auto
#set gfxpayload=keep
#set gfxterm_font=unicode
#load_video
#insmod gfxterm
#terminal_output gfxterm

# Set the default boot entry (first is 0)
set default=0


menuentry "TwitchyLinux" {
        echo "Loading TwitchyLinux..."
        search --no-floppy --fs-uuid --set BOOT_PART_UUID
        linux   /KERN_IMG_FILENAME cryptdevice=UUID=MAIN_PART_UUID:cryptroot root=/dev/mapper/cryptroot apparmor=1 security=apparmor
        initrd /initrd.img-K_VERS
}

menuentry "Linux K_VERS (rescue)" {
        echo "Loading K_VERS in rescue mode..."
        search --no-floppy --fs-uuid --set BOOT_PART_UUID
        linux  /KERN_IMG_FILENAME cryptdevice=UUID=MAIN_PART_UUID:cryptroot root=/dev/mapper/cryptroot systemd.unit=rescue.target
        initrd /initrd.img-K_VERS
}

menuentry "Linux K_VERS (emergency)" {
        echo "Loading K_VERS in emergency mode..."
        search --no-floppy --fs-uuid --set BOOT_PART_UUID
        linux  /KERN_IMG_FILENAME cryptdevice=UUID=MAIN_PART_UUID:cryptroot root=/dev/mapper/cryptroot systemd.unit=emergency.target
        initrd /initrd.img-K_VERS
}

menuentry "System shutdown" {
        echo "System shutting down..."
        halt
}

menuentry "System restart" {
        echo "System rebooting..."
        reboot
}

`
