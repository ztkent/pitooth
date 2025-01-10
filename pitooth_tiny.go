package pitooth

import (
	"fmt"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"tinygo.org/x/bluetooth"
)

type BLEManager interface {
	Start() error
	Stop() error
	AcceptConnections(time.Duration) error
}

type bleManager struct {
	adapter  *bluetooth.Adapter
	adv      bluetooth.Advertisement
	ssid     string
	password string
	status   string
	l        *logrus.Logger
}

type WifiCredentials struct {
	SSID     string
	Password string
}

func NewBLEManager(deviceName string) (BLEManager, error) {
	adapter := bluetooth.DefaultAdapter
	err := adapter.Enable()
	if err != nil {
		return nil, fmt.Errorf("failed to enable BLE: %v", err)
	}

	bm := &bleManager{
		adapter: adapter,
		l:       logrus.New(),
	}

	// Setup Advertisement with complete configuration
	wifiUUID := bluetooth.NewUUID([16]byte{
		0x00, 0x00, 0x18, 0x1A, 0x00, 0x00, 0x10, 0x00,
		0x80, 0x00, 0x00, 0x80, 0x5F, 0x9B, 0x34, 0xFB,
	})
	adv := adapter.DefaultAdvertisement()
	err = adv.Configure(bluetooth.AdvertisementOptions{
		LocalName:    "THisIsADeviceName",
		ServiceUUIDs: []bluetooth.UUID{wifiUUID},
		Interval:     bluetooth.NewDuration(1 * time.Second),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to configure advertisement: %v", err)
	}
	bm.adv = *adv

	// Setup WiFi Service
	err = bm.setupWiFiService()
	if err != nil {
		return nil, fmt.Errorf("failed to setup WiFi service: %v", err)
	}

	return bm, nil
}

func (bm *bleManager) setupWiFiService() error {
	// Standard WiFi Service UUID
	wifiUUID := bluetooth.NewUUID([16]byte{
		0x00, 0x00, 0x18, 0x1A, 0x00, 0x00, 0x10, 0x00,
		0x80, 0x00, 0x00, 0x80, 0x5F, 0x9B, 0x34, 0xFB,
	})
	var ssidChar, passChar, statusChar *bluetooth.Characteristic

	chars := []bluetooth.CharacteristicConfig{
		{
			Handle: ssidChar,
			UUID:   bluetooth.NewUUID([16]byte{0x2A, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}),
			Flags:  bluetooth.CharacteristicReadPermission | bluetooth.CharacteristicWritePermission,
			WriteEvent: func(conn bluetooth.Connection, offset int, data []byte) {
				bm.ssid = string(data)
				bm.l.Infof("SSID set: %s", bm.ssid)
			},
		},
		{
			Handle: passChar,
			UUID:   bluetooth.NewUUID([16]byte{0x2A, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}),
			Flags:  bluetooth.CharacteristicWritePermission | bluetooth.CharacteristicWriteWithoutResponsePermission,
			WriteEvent: func(conn bluetooth.Connection, offset int, data []byte) {
				bm.password = string(data)
				if bm.ssid != "" && bm.password != "" {
					go bm.connectWiFi()
				}
			},
		},
		{
			Handle: statusChar,
			UUID:   bluetooth.NewUUID([16]byte{0x2A, 0x02, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}),
			Flags:  bluetooth.CharacteristicReadPermission | bluetooth.CharacteristicNotifyPermission,
			Value:  []byte("Ready"),
		},
	}

	err := bm.adapter.AddService(&bluetooth.Service{
		UUID:            wifiUUID,
		Characteristics: chars,
	})

	return err
}

func (bm *bleManager) connectWiFi() {
	// TODO: Implement WiFi connection logic
	fmt.Println("Connecting to WiFi...")
}

func (bm *bleManager) Start() error {
	err := bm.adv.Start()
	if err != nil {
		return fmt.Errorf("failed to start advertising: %v", err)
	}
	return nil
}

func (bm *bleManager) Stop() error {
	err := bm.adv.Stop()
	if err != nil {
		return fmt.Errorf("failed to stop advertising: %v", err)
	}
	return nil
}

func (bm *bleManager) AcceptConnections(duration time.Duration) error {
	err := bm.Start()
	if err != nil {
		return err
	}

	time.AfterFunc(duration, func() {
		bm.Stop()
	})

	return nil
}

func TestBLEClientServer(t *testing.T) {
	// Start server
	server, err := NewBLEManager("TestDevice")
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	defer server.Stop()

	// Start client adapter
	clientAdapter := bluetooth.DefaultAdapter
	if err := clientAdapter.Enable(); err != nil {
		t.Fatalf("Failed to enable client adapter: %v", err)
	}

	// Start scanning
	println("Scanning...")
	found := false
	err = clientAdapter.Scan(func(adapter *bluetooth.Adapter, device bluetooth.ScanResult) {
		if device.LocalName() == "TestDevice" {
			found = true
			// Connect to device
			d, err := adapter.Connect(device.Address, bluetooth.ConnectionParams{})
			if err != nil {
				t.Errorf("Failed to connect: %v", err)
				return
			}
			defer d.Disconnect()

			// Discover services
			services, err := d.DiscoverServices(nil)
			if err != nil {
				t.Errorf("Failed to discover services: %v", err)
				return
			}

			// Find WiFi service and characteristics
			for _, svc := range services {
				if svc.UUID().String() == "181a" { // WiFi Service
					chars, err := svc.DiscoverCharacteristics(nil)
					if err != nil {
						t.Errorf("Failed to discover characteristics: %v", err)
						return
					}

					// Test writing SSID and password
					for _, char := range chars {
						switch char.UUID().String() {
						case "2a00": // SSID
							_, err = char.WriteWithoutResponse([]byte("TestSSID"))
							if err != nil {
								t.Errorf("Failed to write SSID: %v", err)
							}
						case "2a01": // Password
							_, err = char.WriteWithoutResponse([]byte("TestPassword"))
							if err != nil {
								t.Errorf("Failed to write password: %v", err)
							}
						}
					}
				}
			}
			adapter.StopScan()
		}
	})

	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	if !found {
		t.Fatal("Device not found")
	}
}
