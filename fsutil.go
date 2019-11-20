package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

type disk struct {
	Name, Path string

	Model    string
	Serial   string
	Bus, Rev string
	Symlinks []string

	NumBlocks int

	Major, Minor int
	PartN        int

	PartTabType string
	PartUUID    string
	FsUUID      string
	Partitions  []*disk
	FS, Label   string
}

var disks []disk

func getDisk(path string) disk {
	path = strings.Split(path, " ")[0]
	for _, d := range disks {
		if d.Path == path {
			return d
		}
	}
	return disk{}
}

func getDevBlockSize(name string) (int, error) {
	d, err := ioutil.ReadFile(fmt.Sprintf("/sys/class/block/%s/size", name))
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.Trim(string(d), "\n\t\r "))
}

func getUdevDiskInfo(path string, isRoot bool) (*disk, error) {
	c := exec.Command("udevadm", "info", "-q", "all", "--name", path)
	o, err := c.Output()
	if err != nil {
		return nil, err
	}
	r := bufio.NewScanner(bytes.NewReader(o))
	out := disk{Path: path}

	for r.Scan() {
		line := r.Text()
		if len(line) < 4 {
			continue
		}

		switch line[:3] {
		case "N: ":
			out.Name = line[3:]
		case "S: ":
			out.Symlinks = append(out.Symlinks, line[3:])
		case "E: ":
			var err error
			if strings.HasPrefix(line, "E: ID_MODEL=") {
				out.Model = line[len("E: ID_MODEL="):]
			} else if strings.HasPrefix(line, "E: ID_PART_TABLE_TYPE=") {
				out.PartTabType = line[len("E: ID_PART_TABLE_TYPE="):]
			} else if strings.HasPrefix(line, "E: ID_PART_TABLE_UUID=") {
				out.PartUUID = line[len("E: ID_PART_TABLE_UUID="):]
			} else if strings.HasPrefix(line, "E: ID_SERIAL=") {
				out.Serial = line[len("E: ID_SERIAL="):]
			} else if strings.HasPrefix(line, "E: ID_REVISION=") {
				out.Rev = line[len("E: ID_REVISION="):]
			} else if strings.HasPrefix(line, "E: ID_BUS=") {
				out.Bus = line[len("E: ID_BUS="):]
			} else if strings.HasPrefix(line, "E: ID_FS_TYPE=") {
				out.FS = line[len("E: ID_FS_TYPE="):]
			} else if strings.HasPrefix(line, "E: ID_FS_LABEL=") {
				out.Label = line[len("E: ID_FS_LABEL="):]
			} else if strings.HasPrefix(line, "E: ID_FS_UUID=") {
				out.FsUUID = line[len("E: ID_FS_UUID="):]
			} else if strings.HasPrefix(line, "E: MAJOR=") {
				out.Major, err = strconv.Atoi(line[len("E: MAJOR="):])
				if err != nil {
					return nil, fmt.Errorf("decoding major: %v", err)
				}
			} else if strings.HasPrefix(line, "E: MINOR=") {
				out.Minor, err = strconv.Atoi(line[len("E: MINOR="):])
				if err != nil {
					return nil, fmt.Errorf("decoding minor: %v", err)
				}
			} else if strings.HasPrefix(line, "E: PARTN=") {
				out.PartN, err = strconv.Atoi(line[len("E: PARTN="):])
				if err != nil {
					return nil, fmt.Errorf("decoding partN: %v", err)
				}
			}
		}
	}

	if out.Major != 0 && isRoot {
		for i := 1; i < 12; i++ {
			part, err := getUdevDiskInfo(fmt.Sprintf("%s%d", path, i), false)
			if err != nil {
				if _, ok := err.(*exec.ExitError); ok {
					return &out, nil
				}
				return nil, err
			}
			if part.PartN != 0 {
				out.Partitions = append(out.Partitions, part)
			}
		}
	}

	return &out, nil
}

func getDiskInfo() ([]disk, error) {
	stdout, err := exec.Command("lsblk", "-Jadp").Output()
	if err != nil {
		return nil, err
	}
	var blockDevs map[string][]struct {
		Name string `json:"name"`
		Size string `json:"size"`
		Type string `json:"type"`
	}
	if err := json.Unmarshal(stdout, &blockDevs); err != nil {
		fmt.Println(string(stdout))
		return nil, err
	}

	var out []disk
	for _, blkDev := range blockDevs["blockdevices"] {
		if blkDev.Type == "disk" {
			diskInfo, err := getUdevDiskInfo(blkDev.Name, true)
			if err != nil {
				return nil, err
			}
			diskInfo.NumBlocks, err = getDevBlockSize(diskInfo.Name)
			if err != nil {
				return nil, err
			}
			out = append(out, *diskInfo)
		}
	}

	return out, nil
}

func byteCountDecimal(b int64) string {
	const unit = 1000
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "kMGTPE"[exp])
}

func readDiskInfo(mw *mainWindow) {
	mw.setDebugValue([]string{"disks"}, "")
	var err error
	disks, err = getDiskInfo()
	if err != nil {
		fmt.Fprintf(os.Stderr, "getDiskInfo() failed: %v", err)
		return
	}
	for _, disk := range disks {
		mw.setDebugValue([]string{"disks", disk.Name}, "")
		mw.setDebugValue([]string{"disks", disk.Name, "Bus"}, disk.Bus)
		mw.setDebugValue([]string{"disks", disk.Name, "Model"}, disk.Model)
		mw.setDebugValue([]string{"disks", disk.Name, "Serial"}, disk.Serial)
		mw.setDebugValue([]string{"disks", disk.Name, "Size"}, byteCountDecimal(int64(disk.NumBlocks*512)))
		mw.setDebugValue([]string{"disks", disk.Name, "Revision"}, disk.Rev)
		mw.setDebugValue([]string{"disks", disk.Name, "Partition Table"}, disk.PartTabType)

		if len(disk.Partitions) > 0 {
			mw.setDebugValue([]string{"disks", disk.Name, "Partitions"}, "")
			for _, part := range disk.Partitions {
				mw.setDebugValue([]string{"disks", disk.Name, "Partitions", fmt.Sprint(part.PartN)}, "")
				bs, err := getDevBlockSize(part.Name)
				if err != nil {
					mw.setDebugValue([]string{"disks", disk.Name, "Partitions", fmt.Sprint(part.PartN), "Size"}, "Failed to read size: "+err.Error())
				} else {
					mw.setDebugValue([]string{"disks", disk.Name, "Partitions", fmt.Sprint(part.PartN), "Size"}, byteCountDecimal(int64(bs*512)))
				}
				mw.setDebugValue([]string{"disks", disk.Name, "Partitions", fmt.Sprint(part.PartN), "Label"}, part.Label)
				mw.setDebugValue([]string{"disks", disk.Name, "Partitions", fmt.Sprint(part.PartN), "Filesystem"}, part.FS)
				mw.setDebugValue([]string{"disks", disk.Name, "Partitions", fmt.Sprint(part.PartN), "FS UUID"}, part.FsUUID)
			}
		}
	}
	mw.setDisks(disks)
}
