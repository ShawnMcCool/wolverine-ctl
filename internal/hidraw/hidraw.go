// Package hidraw finds Linux hidraw nodes by USB identity and exchanges HID
// feature reports with them. It uses only sysfs and the hidraw ioctls from
// linux/hidraw.h; no kernel driver is detached.
package hidraw

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

// Node is one hidraw device node and the USB interface it belongs to.
type Node struct {
	Path      string // /dev/hidrawN
	VendorID  uint16
	ProductID uint16
	Interface int    // USB bInterfaceNumber
	USBPath   string // sysfs USB device directory, e.g. /sys/devices/.../1-5
	Product   string // USB product string, may be empty
}

const sysClass = "/sys/class/hidraw"

// Discover lists every hidraw node backed by a USB interface.
func Discover() ([]Node, error) {
	entries, err := os.ReadDir(sysClass)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", sysClass, err)
	}
	var nodes []Node
	for _, entry := range entries {
		node, ok := describe(entry.Name())
		if ok {
			nodes = append(nodes, node)
		}
	}
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].Path < nodes[j].Path })
	return nodes, nil
}

// describe resolves hidrawN/device -> HID device -> USB interface -> USB device.
func describe(name string) (Node, bool) {
	hid, err := filepath.EvalSymlinks(filepath.Join(sysClass, name, "device"))
	if err != nil {
		return Node{}, false
	}
	intf := filepath.Dir(hid)
	usb := filepath.Dir(intf)
	number, err := readHex(filepath.Join(intf, "bInterfaceNumber"))
	if err != nil {
		return Node{}, false
	}
	vendor, err := readHex(filepath.Join(usb, "idVendor"))
	if err != nil {
		return Node{}, false
	}
	product, err := readHex(filepath.Join(usb, "idProduct"))
	if err != nil {
		return Node{}, false
	}
	return Node{
		Path:      filepath.Join("/dev", name),
		VendorID:  uint16(vendor),
		ProductID: uint16(product),
		Interface: int(number),
		USBPath:   usb,
		Product:   readText(filepath.Join(usb, "product")),
	}, true
}

func readText(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func readHex(path string) (uint64, error) {
	text := readText(path)
	if text == "" {
		return 0, fmt.Errorf("%s: empty", path)
	}
	return strconv.ParseUint(text, 16, 16)
}

// Device is an open hidraw node holding an exclusive advisory lock, so two
// wolverine-ctl processes never interleave commands on one controller.
type Device struct {
	file *os.File
}

// Open opens path read-write and takes the advisory lock, waiting up to wait.
func Open(path string, wait time.Duration) (*Device, error) {
	file, err := os.OpenFile(path, os.O_RDWR|syscall.O_CLOEXEC, 0)
	if err != nil {
		if errors.Is(err, os.ErrPermission) {
			return nil, fmt.Errorf("opening %s: permission denied; install the udev rule (see README) and reconnect the receiver", path)
		}
		return nil, fmt.Errorf("opening %s: %w", path, err)
	}
	deadline := time.Now().Add(wait)
	for {
		err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			return &Device{file: file}, nil
		}
		if !errors.Is(err, syscall.EWOULDBLOCK) || time.Now().After(deadline) {
			file.Close()
			return nil, fmt.Errorf("locking %s: %w (another wolverine-ctl is using it)", path, err)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// Close releases the lock and the file.
func (d *Device) Close() error { return d.file.Close() }

// ioctl request numbers, _IOC(_IOC_READ|_IOC_WRITE, 'H', nr, len).
func featureRequest(nr uintptr, length int) uintptr {
	const iocReadWrite = 3
	return iocReadWrite<<30 | uintptr(length)<<16 | uintptr('H')<<8 | nr
}

// SetFeature sends report (report ID in byte 0) with HIDIOCSFEATURE.
func (d *Device) SetFeature(report []byte) error {
	return d.ioctl(featureRequest(0x06, len(report)), report)
}

// GetFeature reads a feature report into report; set report[0] to the report ID
// first. Uses HIDIOCGFEATURE.
func (d *Device) GetFeature(report []byte) error {
	return d.ioctl(featureRequest(0x07, len(report)), report)
}

func (d *Device) ioctl(request uintptr, buffer []byte) error {
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, d.file.Fd(), request, uintptr(unsafe.Pointer(&buffer[0])))
	if errno != 0 {
		return errno
	}
	return nil
}
