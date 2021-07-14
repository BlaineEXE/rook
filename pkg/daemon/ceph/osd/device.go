/*
Copyright 2016 The Rook Authors. All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package osd

import (
	"encoding/binary"
	"encoding/json"
	"io"
	"os"
	"path"

	"github.com/pkg/errors"
)

const (
	bootstrapOSDKeyringTemplate = `
[client.bootstrap-osd]
	key = %s
	caps mon = "allow profile bootstrap-osd"
`
)

// Device is a device
type Device struct {
	Name   string `json:"name"`
	NodeID string `json:"nodeId"`
	Dir    bool   `json:"bool"`
}

// DesiredDevice keeps track of the desired settings for a device
type DesiredDevice struct {
	Name               string
	OSDsPerDevice      int
	MetadataDevice     string
	DatabaseSizeMB     int
	DeviceClass        string
	InitialWeight      string
	IsFilter           bool
	IsDevicePathFilter bool
}

// DeviceOsdMapping represents the mapping of an OSD on disk
type DeviceOsdMapping struct {
	Entries map[string]*DeviceOsdIDEntry // device name to OSD ID mapping entry
}

// DeviceOsdIDEntry represents the details of an OSD
type DeviceOsdIDEntry struct {
	Data                  int           // OSD ID that has data stored here
	Metadata              []int         // OSD IDs (multiple) that have metadata stored here
	Config                DesiredDevice // Device specific config options
	PersistentDevicePaths []string
}

func (m *DeviceOsdMapping) String() string {
	b, _ := json.Marshal(m)
	return string(b)
}

// allow this to be overridden for unit testing
var device deviceReader = osDeviceReader{}

type deviceReader interface {
	OpenFile(name string, flag int, perm os.FileMode) (file, error)
}

type file interface {
	io.Closer
	io.Reader
}

// osDeviceReader implements deviceReader using the local disk.
type osDeviceReader struct{}

func (osDeviceReader) OpenFile(name string, flag int, perm os.FileMode) (file, error) {
	return os.OpenFile(name, flag, perm)
}

// return true if the device has a bluestore header indicating it is a bluestore OSD
func hasBluestoreHeader(deviceName string) (bool, error) {
	devicePath := path.Join("/dev", deviceName)

	dev, err := device.OpenFile(devicePath, os.O_RDONLY, os.ModeDevice)
	if err != nil {
		return false, errors.Wrapf(err, "failed to check if device %q has a bluestore header", devicePath)
	}
	defer dev.Close()

	// check for the bluestore header "bluestore block device" (22 bytes long)
	// see: https://github.com/ceph/ceph/blob/4dae3915a842281f93486b612f645eb2eb604385/src/os/bluestore/bluestore_types.cc#L35
	// code based on: https://github.com/rekby/gpt/blob/7da10aec5566349f29875dad4a59c8341b01e00a/gpt.go#L81
	sig := [22]byte{}
	err = binary.Read(dev, binary.LittleEndian, &sig)
	if err != nil {
		return false, errors.Wrapf(err, "failed to read bluestore header from device %q", devicePath)
	}
	if string(sig[:]) == "bluestore block device" {
		return true, nil
	}

	return false, nil
}
