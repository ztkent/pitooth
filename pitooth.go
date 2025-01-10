package pitooth

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/muka/go-bluetooth/bluez/profile/adapter"
	"github.com/muka/go-bluetooth/bluez/profile/agent"
	"github.com/sirupsen/logrus"
)

/*
	PiTooth is a simple Bluetooth manager for Raspberry Pi devices.
	- Accept incoming connections
	- Get a list of nearby/connected devices
	- Control the OBEX server
	- Receive files from connected devices
*/

func init() {
	// Suppress excess warning logs from the bluetooth library
	logrus.SetLevel(logrus.ErrorLevel)
}

type BluetoothManager interface {
	AcceptConnections(time.Duration) error
	GetNearbyDevices() (map[string]Device, error)
	GetAdapter() *adapter.Adapter1
	// OBEX is a protocol for transferring files between devices over Bluetooth
	ControlOBEXServer(bool, string) error

	Start()
	Stop()
}

type bluetoothManager struct {
	agent *PiToothAgent
	l     *logrus.Logger
	*adapter.Adapter1
}

type Device struct {
	LastSeen  time.Time
	Address   string
	Name      string
	Connected bool
}

func NewBluetoothManager(deviceAlias string, opts ...BluetoothManagerOption) (BluetoothManager, error) {
	// We should always set a device alias, or it gets tricky.
	if deviceAlias == "" {
		return nil, fmt.Errorf("Bluetooth device alias cannot be empty")
	}

	// Only support Linux, this should be running on a Raspberry Pi
	if runtime.GOOS != "linux" {
		return nil, fmt.Errorf("Unsupported OS: %v", runtime.GOOS)
	} else {
		_, err := os.Stat("/proc/device-tree/model")
		if err != nil {
			return nil, fmt.Errorf("Not a Raspberry Pi, can't enable Bluetooth: %v", err)
		}
	}

	// Get the bt adapter to manage bluetooth devices
	defaultAdapter, err := adapter.GetDefaultAdapter()
	if err != nil {
		return nil, fmt.Errorf("Failed to get default adapter: %v", err)
	}

	// Connect pitooth agent to handle pairing requests
	pitoothAgent := &PiToothAgent{
		SimpleAgent: agent.NewSimpleAgent(),
		l:           defaultLogger(),
	}

	btm := bluetoothManager{
		Adapter1: defaultAdapter,
		agent:    pitoothAgent,
		l:        defaultLogger(),
	}

	// Apply any options
	for _, opt := range opts {
		err := opt(&btm)
		if err != nil {
			return nil, err
		}
	}

	// Apply the registration agent to the adapter
	err = agent.ExposeAgent(btm.Client().GetConnection(), btm.agent, agent.CapNoInputNoOutput, true)
	if err != nil {
		return nil, fmt.Errorf("Failed to register agent: %v", err)
	}

	// Set the device alias
	err = btm.SetAlias(deviceAlias)
	if err != nil {
		return nil, fmt.Errorf("Failed to set bluetooth alias: %v", err)
	}
	err = btm.SetPowered(true)
	if err != nil {
		return nil, fmt.Errorf("Failed to power on bluetooth adapter: %v", err)
	}
	return &btm, nil
}

type BluetoothManagerOption func(*bluetoothManager) error

// WithLogger configures a custom logger for the Bluetooth manager
func WithLogger(l *logrus.Logger) BluetoothManagerOption {
	return func(bm *bluetoothManager) error {
		bm.l = l
		bm.agent.l = l
		return nil
	}
}

// WithAdapter configures a custom Bluetooth adapter that implements the Adapter1 interface
func WithAdapter(a adapter.Adapter1) BluetoothManagerOption {
	return func(bm *bluetoothManager) error {
		bm.Adapter1 = &a
		return nil
	}
}

// Opens the bluetooth adapter to accept connections for a period of time
func (btm *bluetoothManager) AcceptConnections(pairingWindow time.Duration) error {
	btm.l.Debugln("PiTooth: Starting Pairing...")
	if pairingWindow == 0 {
		btm.l.Debugln("PiTooth: No pairing window specified, defaulting to 30 seconds")
		pairingWindow = 30 * time.Second
	}

	// Make the device discoverable
	btm.l.Debugln("PiTooth: Checking power is on...")
	btm.SetPowered(true)

	btm.l.Debugln("PiTooth: Setting Pairable...")
	err := btm.SetPairable(true)
	if err != nil {
		return fmt.Errorf("Failed to make device pairable: %v", err)
	}

	// Make the device discoverable
	btm.l.Debugln("PiTooth: Setting Discoverable...")
	err = btm.SetDiscoverable(true)
	if err != nil {
		return fmt.Errorf("Failed to make device discoverable: %v", err)
	}

	// Waiting for device connections
	btm.l.Infoln("PiTooth: Accepting Connections...")
	start := time.Now()
	for time.Since(start) < pairingWindow {
		time.Sleep(1 * time.Second)
	}

	// Make the device undiscoverable
	btm.l.Debugln("PiTooth: Setting Undiscoverable...")
	err = btm.SetDiscoverable(false)
	if err != nil {
		return fmt.Errorf("Failed to make device undiscoverable: %v", err)
	}
	return nil
}

// Get a map of all the nearby devices
func (btm *bluetoothManager) GetNearbyDevices() (map[string]Device, error) {
	btm.l.Debugln("PiTooth: Starting GetNearbyDevices...")
	nearbyDevices, err := btm.collectNearbyDevices()
	if err != nil {
		return nil, err
	}

	btm.l.Debugln("PiTooth: # of nearby devices: ", len(nearbyDevices))
	for _, device := range nearbyDevices {
		btm.l.Debugln("PiTooth: Nearby device: ", device.Name, " : ", device.Address, " : ", device.LastSeen, " : ", device.Connected)
	}
	return nearbyDevices, nil
}

// Get the devices every second, for 5 seconds.
// Return a map of all the devices found.
func (btm *bluetoothManager) collectNearbyDevices() (map[string]Device, error) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	done := time.After(5 * time.Second)

	nearbyDevices := make(map[string]Device)
	for {
		select {
		case <-done:
			return nearbyDevices, nil
		case <-ticker.C:
			devices, err := btm.GetDevices()
			if err != nil {
				return nil, fmt.Errorf("Failed to get bluetooth devices: %v", err)
			}
			for _, device := range devices {
				btm.l.Debugln("PiTooth: Discovered bluetooth device: ", device.Properties.Alias, " : ", device.Properties.Address)
				nearbyDevices[device.Properties.Address] = Device{
					LastSeen:  time.Now(),
					Address:   device.Properties.Address,
					Name:      device.Properties.Alias,
					Connected: device.Properties.Connected,
				}
			}
		}
	}
}

func (btm *bluetoothManager) Start() {
	btm.SetPowered(true)
	btm.SetPairable(true)
	btm.SetDiscoverable(true)
}

// Close the active bluetooth adapter & agent
// Optionally turn off the bluetooth device
func (btm *bluetoothManager) Stop() {
	btm.SetDiscoverable(false)
	btm.SetPairable(false)
	btm.SetPowered(false)
}

func (btm *bluetoothManager) GetAdapter() *adapter.Adapter1 {
	return btm.Adapter1
}

func defaultLogger() *logrus.Logger {
	l := logrus.New()
	// Setup the logger, so it can be parsed by datadog
	l.Formatter = &logrus.JSONFormatter{}
	l.SetOutput(os.Stdout)
	// Set the log level
	logLevel := strings.ToLower(os.Getenv("LOG_LEVEL"))
	switch logLevel {
	case "debug":
		l.SetLevel(logrus.DebugLevel)
	case "info":
		l.SetLevel(logrus.InfoLevel)
	case "error":
		l.SetLevel(logrus.ErrorLevel)
	default:
		l.SetLevel(logrus.InfoLevel)
	}
	return l
}
