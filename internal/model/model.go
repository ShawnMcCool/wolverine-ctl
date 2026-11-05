// Package model lists the controllers wolverine-ctl supports.
//
// To add a controller that speaks the same protocol, add an entry to Models
// with the evidence for it in Source, add its USB ID to the udev rule, and
// list it in README.md and SOURCES.md.
package model

import "github.com/ShawnMcCool/wolverine-ctl/internal/mapping"

const RazerVendorID = 0x1532

// Model is one USB identity of a supported controller.
type Model struct {
	Name      string
	VendorID  uint16
	ProductID uint16
	// ConfigInterface is the USB interface whose hidraw node carries the
	// configuration feature report.
	ConfigInterface int
	PollingRates    []int
	Assignments     mapping.Dialect
	// Verified is true when wolverine-ctl itself was tested on this model.
	Verified bool
	Source   string
}

var allRates = []int{250, 500, 1000, 2000, 4000, 8000}

var Models = []Model{
	{
		Name:            "Razer Wolverine V3 Pro 8K PC (2.4 GHz receiver)",
		VendorID:        RazerVendorID,
		ProductID:       0x0a59,
		ConfigInterface: 1,
		PollingRates:    allRates,
		Assignments:     mapping.Dialect{TargetZeroDisables: true},
		Verified:        true,
		Source:          "Tested with wolverine-ctl, firmware 1.4.0, 2026-11-05",
	},
	{
		Name:            "Razer Wolverine V4 Pro (PC HID mode)",
		VendorID:        RazerVendorID,
		ProductID:       0x0a79,
		ConfigInterface: 1,
		PollingRates:    allRates,
		Assignments:     mapping.Dialect{TargetZeroDisables: false},
		Verified:        false,
		Source:          "Same protocol, verified by hyprdeck-razer; not tested with wolverine-ctl",
	},
}

// Lookup returns the model with this USB identity.
func Lookup(vendorID, productID uint16) (Model, bool) {
	for _, m := range Models {
		if m.VendorID == vendorID && m.ProductID == productID {
			return m, true
		}
	}
	return Model{}, false
}

// SupportsRate reports whether rate is one of the model's polling rates.
func (m Model) SupportsRate(rate int) bool {
	for _, r := range m.PollingRates {
		if r == rate {
			return true
		}
	}
	return false
}
