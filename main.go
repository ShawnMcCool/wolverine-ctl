// Command wolverine-ctl configures Razer Wolverine controllers on Linux through
// their onboard settings: polling rate and button assignments.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"runtime/debug"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/ShawnMcCool/wolverine-ctl/internal/hidraw"
	"github.com/ShawnMcCool/wolverine-ctl/internal/mapping"
	"github.com/ShawnMcCool/wolverine-ctl/internal/model"
	"github.com/ShawnMcCool/wolverine-ctl/internal/protocol"
)

// version is set by 'make' with -ldflags; 'go install' builds fall back to
// the module version recorded in the binary.
var version = "dev"

func init() {
	if info, ok := debug.ReadBuildInfo(); ok && version == "dev" && info.Main.Version != "" && info.Main.Version != "(devel)" {
		version = info.Main.Version
	}
}

// command is one subcommand. To add one, write its run function and add an
// entry to commands.
type command struct {
	name    string
	args    string
	summary string
	// needsController commands get an open controller session.
	needsController bool
	run             func(ctx *context, args []string) error
}

var commands = []command{
	{"list", "", "List connected supported controllers", false, runList},
	{"status", "", "Show firmware, battery, polling rate and button assignments", true, runStatus},
	{"polling", "[RATE]", "Show or set the polling rate in Hz", true, runPolling},
	{"map", "BUTTON TARGET", "Assign a button: default | off | button NAME | key CHORD", true, runMap},
	{"names", "", "List button, controller-target, key and modifier names", false, runNames},
	{"version", "", "Print the version", false, func(*context, []string) error { fmt.Println(version); return nil }},
}

// context is what a command runs with.
type context struct {
	devicePath string // serial number or /dev/hidrawN
	profile    int    // 0 means the active profile
	controller *controller
}

type controller struct {
	node   hidraw.Node
	model  model.Model
	device *hidraw.Device
	client *protocol.Client
}

func main() {
	flags := flag.NewFlagSet("wolverine-ctl", flag.ContinueOnError)
	flags.Usage = func() { usage(flags) }
	ctx := &context{}
	flags.StringVar(&ctx.devicePath, "device", "", "controller to use: serial number or hidraw path, see 'list' (default: the one responding controller)")
	flags.IntVar(&ctx.profile, "profile", 0, "onboard profile to read or change (default: the active profile)")
	if err := flags.Parse(os.Args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return
		}
		os.Exit(2)
	}
	if flags.NArg() == 0 {
		usage(flags)
		os.Exit(2)
	}
	name, args := flags.Arg(0), flags.Args()[1:]
	cmd, ok := find(name)
	if !ok {
		fmt.Fprintf(os.Stderr, "wolverine-ctl: unknown command %q\n\n", name)
		usage(flags)
		os.Exit(2)
	}
	if err := execute(ctx, cmd, args); err != nil {
		fmt.Fprintln(os.Stderr, "wolverine-ctl:", err)
		os.Exit(1)
	}
}

func find(name string) (command, bool) {
	for _, c := range commands {
		if c.name == name {
			return c, true
		}
	}
	return command{}, false
}

func execute(ctx *context, cmd command, args []string) error {
	if cmd.needsController {
		c, err := openController(ctx.devicePath)
		if err != nil {
			return err
		}
		defer c.device.Close()
		ctx.controller = c
	}
	return cmd.run(ctx, args)
}

func usage(flags *flag.FlagSet) {
	out := flags.Output()
	fmt.Fprintln(out, "Usage: wolverine-ctl [--device SERIAL|PATH] [--profile N] COMMAND [ARGS]")
	fmt.Fprintln(out, "\nCommands:")
	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	for _, c := range commands {
		fmt.Fprintf(w, "  %s %s\t%s\n", c.name, c.args, c.summary)
	}
	w.Flush()
	fmt.Fprintln(out, "\nFlags:")
	flags.PrintDefaults()
	fmt.Fprintln(out, "\nExamples:")
	fmt.Fprintln(out, "  wolverine-ctl status")
	fmt.Fprintln(out, "  wolverine-ctl polling 2000")
	fmt.Fprintln(out, "  wolverine-ctl map m3 key f13")
	fmt.Fprintln(out, "  wolverine-ctl map m4 key ctrl+shift+s")
	fmt.Fprintln(out, "  wolverine-ctl map m5 button a")
	fmt.Fprintln(out, "  wolverine-ctl map m6 off")
}

// supported returns connected hidraw nodes that are a supported model's
// configuration interface.
func supported() ([]hidraw.Node, error) {
	nodes, err := hidraw.Discover()
	if err != nil {
		return nil, err
	}
	var out []hidraw.Node
	for _, n := range nodes {
		if m, ok := model.Lookup(n.VendorID, n.ProductID); ok && n.Interface == m.ConfigInterface {
			out = append(out, n)
		}
	}
	return out, nil
}

// openController picks the controller to talk to. selector is empty, a
// hidraw path (/dev/hidrawN) or a serial number.
func openController(selector string) (*controller, error) {
	nodes, err := supported()
	if err != nil {
		return nil, err
	}
	if len(nodes) == 0 {
		return nil, errors.New("no supported controller connected; run 'wolverine-ctl list' and see README for supported models")
	}
	if strings.HasPrefix(selector, "/dev/") {
		for _, n := range nodes {
			if n.Path == selector {
				return open(n)
			}
		}
		return nil, fmt.Errorf("%s is not a supported controller's configuration interface; run 'wolverine-ctl list'", selector)
	}
	if selector == "" && len(nodes) == 1 {
		return open(nodes[0])
	}
	return answering(nodes, selector)
}

func open(node hidraw.Node) (*controller, error) {
	m, _ := model.Lookup(node.VendorID, node.ProductID)
	device, err := hidraw.Open(node.Path, time.Second)
	if err != nil {
		return nil, err
	}
	return &controller{node: node, model: m, device: device, client: protocol.NewClient(device)}, nil
}

// probe is one configuration interface and the serial of the controller
// answering on it.
type probe struct {
	node   hidraw.Node
	serial string
	err    error
}

// probeAll asks every interface for its controller's serial number. A
// controller plugged in by cable is also listed under its receiver, which
// then answers "no response" (status 0x04).
func probeAll(nodes []hidraw.Node) []probe {
	out := make([]probe, 0, len(nodes))
	for _, n := range nodes {
		p := probe{node: n}
		c, err := open(n)
		if err == nil {
			p.serial, p.err = c.client.Serial()
			c.device.Close()
		} else {
			p.err = err
		}
		out = append(out, p)
	}
	return out
}

// answering opens the one responding controller, limited to serial when set.
func answering(nodes []hidraw.Node, serial string) (*controller, error) {
	var matches []probe
	for _, p := range probeAll(nodes) {
		if p.err == nil && (serial == "" || strings.EqualFold(p.serial, serial)) {
			matches = append(matches, p)
		}
	}
	switch {
	case len(matches) == 1:
		return open(matches[0].node)
	case serial != "" && len(matches) == 0:
		return nil, fmt.Errorf("no responding controller with serial %s; run 'wolverine-ctl list'", serial)
	case len(matches) == 0:
		return nil, errors.New("no connected controller responded; it may be asleep or off, press a button and retry")
	default:
		return nil, errors.New("more than one controller is responding; choose one with --device SERIAL (see 'wolverine-ctl list')")
	}
}

func (ctx *context) resolveProfile() (byte, error) {
	if ctx.profile < 0 || ctx.profile > 255 {
		return 0, fmt.Errorf("invalid profile %d", ctx.profile)
	}
	if ctx.profile != 0 {
		return byte(ctx.profile), nil
	}
	return ctx.controller.client.ActiveProfile()
}

func runList(_ *context, args []string) error {
	if len(args) != 0 {
		return errors.New("list takes no arguments")
	}
	nodes, err := supported()
	if err != nil {
		return err
	}
	if len(nodes) == 0 {
		fmt.Println("No supported controller connected.")
		return nil
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "DEVICE\tUSB ID\tSERIAL\tCONTROLLER")
	for _, p := range probeAll(nodes) {
		m, _ := model.Lookup(p.node.VendorID, p.node.ProductID)
		serial := p.serial
		if p.err != nil {
			serial = "(not responding)"
		}
		fmt.Fprintf(w, "%s\t%04x:%04x\t%s\t%s%s\n", p.node.Path, p.node.VendorID, p.node.ProductID, serial, m.Name, untested(m))
	}
	return w.Flush()
}

func untested(m model.Model) string {
	if m.Verified {
		return ""
	}
	return " (untested with wolverine-ctl)"
}

func runStatus(ctx *context, args []string) error {
	if len(args) != 0 {
		return errors.New("status takes no arguments")
	}
	c := ctx.controller
	client := c.client
	// The first command fails with a clear message when the controller is
	// asleep, instead of printing a page of errors.
	profile, err := ctx.resolveProfile()
	if err != nil {
		return err
	}
	fmt.Printf("%s%s  %s\n", c.model.Name, untested(c.model), c.node.Path)
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	row := func(label string, value string, err error) {
		if err != nil {
			value = "unavailable: " + err.Error()
		}
		fmt.Fprintf(w, "  %s\t%s\n", label, value)
	}
	firmware, err := client.Firmware()
	row("firmware", firmware.String(), err)
	serial, err := client.Serial()
	row("serial", serial, err)
	battery, err := client.Battery()
	charging, chargeErr := client.Charging()
	batteryText := fmt.Sprintf("%d%%", battery)
	if chargeErr == nil {
		batteryText += " (" + charging.String() + ")"
	}
	row("battery", batteryText, err)
	row("profile", strconv.Itoa(int(profile)), nil)
	rate, err := client.PollingRate(profile)
	row("polling", fmt.Sprintf("%d Hz", rate), err)
	ids, err := client.Buttons()
	if err != nil {
		row("buttons", "", err)
		return w.Flush()
	}
	for _, id := range ids {
		record, err := client.Assignment(profile, id)
		row(mapping.ButtonName(id), mapping.Decode(record, c.model.Assignments).String(), err)
	}
	return w.Flush()
}

func runPolling(ctx *context, args []string) error {
	if len(args) > 1 {
		return errors.New("usage: polling [RATE]")
	}
	client := ctx.controller.client
	profile, err := ctx.resolveProfile()
	if err != nil {
		return err
	}
	if len(args) == 0 {
		rate, err := client.PollingRate(profile)
		if err != nil {
			return err
		}
		fmt.Printf("%d Hz (profile %d)\n", rate, profile)
		return nil
	}
	rate, err := strconv.Atoi(strings.TrimSuffix(strings.ToLower(args[0]), "hz"))
	if err != nil || !ctx.controller.model.SupportsRate(rate) {
		return fmt.Errorf("invalid rate %q; choose one of %v", args[0], ctx.controller.model.PollingRates)
	}
	if err := client.SetPollingRate(profile, rate); err != nil {
		return err
	}
	got, err := client.PollingRate(profile)
	if err != nil {
		return fmt.Errorf("rate sent but readback failed: %w", err)
	}
	if got != rate {
		return fmt.Errorf("rate sent but controller reports %d Hz", got)
	}
	fmt.Printf("%d Hz (profile %d)\n", got, profile)
	return nil
}

func runMap(ctx *context, args []string) error {
	if len(args) < 2 {
		return errors.New("usage: map BUTTON TARGET (TARGET: default | off | button NAME | key CHORD)")
	}
	client := ctx.controller.client
	button, ok := mapping.ButtonID(args[0])
	if !ok {
		return fmt.Errorf("unknown button %q; run 'wolverine-ctl names'", args[0])
	}
	dialect := ctx.controller.model.Assignments
	assignment, err := mapping.Parse(args[1:], dialect)
	if err != nil {
		return err
	}
	ids, err := client.Buttons()
	if err != nil {
		return err
	}
	if !contains(ids, button) {
		return fmt.Errorf("this controller has no programmable button %q", args[0])
	}
	profile, err := ctx.resolveProfile()
	if err != nil {
		return err
	}
	if err := client.SetAssignment(assignment.Record(profile, button, dialect)); err != nil {
		return err
	}
	record, err := client.Assignment(profile, button)
	if err != nil {
		return fmt.Errorf("assignment sent but readback failed: %w", err)
	}
	if !assignment.Matches(record, dialect) {
		return fmt.Errorf("assignment sent but controller reports %s", mapping.Decode(record, dialect))
	}
	fmt.Printf("%s: %s (profile %d)\n", mapping.ButtonName(button), mapping.Decode(record, dialect), profile)
	return nil
}

func contains(ids []byte, id byte) bool {
	for _, v := range ids {
		if v == id {
			return true
		}
	}
	return false
}

func runNames(_ *context, args []string) error {
	if len(args) != 0 {
		return errors.New("names takes no arguments")
	}
	buttons, targets, keys, modifiers := mapping.Names()
	fmt.Println("Buttons:            ", strings.Join(buttons, " "))
	fmt.Println("Controller targets: ", strings.Join(targets, " "))
	fmt.Println("Modifiers:          ", strings.Join(modifiers, " "))
	fmt.Println("Keys:               ", strings.Join(keys, " "))
	return nil
}
