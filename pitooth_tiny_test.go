package pitooth

import (
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"tinygo.org/x/bluetooth"
)

func TestBLEManagerDebug(t *testing.T) {
	// Setup debug logger
	l := logrus.New()
	l.SetLevel(logrus.DebugLevel)

	// Test device setup
	t.Run("Setup_BLE_Adapter", func(t *testing.T) {
		bm, err := NewBLEManager("TestDevice")
		if err != nil {
			t.Logf("BLE Setup Failed: %v", err)
			t.FailNow()
		}
		defer bm.Stop()

		// Log adapter state
		adp := bm.(*bleManager).adapter
		t.Logf("Adapter Enabled: %v", adp != nil)
		t.Logf("Device Name: TestDevice")
	})

	// Test advertising
	t.Run("Test_Advertising", func(t *testing.T) {
		bm, _ := NewBLEManager("TestDevice")
		defer bm.Stop()

		// Start advertising
		err := bm.Start()
		if err != nil {
			t.Logf("Advertising Start Failed: %v", err)
			t.FailNow()
		}
		t.Log("Advertising Started")

		// Wait to verify advertisement
		time.Sleep(5 * time.Second)

		// Stop advertising
		err = bm.Stop()
		if err != nil {
			t.Logf("Advertising Stop Failed: %v", err)
			t.FailNow()
		}
		t.Log("Advertising Stopped")
	})

	// Test WiFi service
	t.Run("Test_WiFi_Service", func(t *testing.T) {
		bm, _ := NewBLEManager("TestDevice")
		defer bm.Stop()

		mgr := bm.(*bleManager)

		// Verify service setup
		t.Logf("SSID Value: %v", mgr.ssid)
		t.Logf("Password Value: %v", mgr.password)
		t.Logf("Status Value: %v", mgr.status)

		// Start advertising for manual testing
		err := bm.AcceptConnections(30 * time.Second)
		if err != nil {
			t.Logf("Connection Accept Failed: %v", err)
			t.FailNow()
		}

		t.Log("Advertising for 30 seconds - Use nRF Connect to test characteristics")
		time.Sleep(30 * time.Second)
	})
}

func TestLongRunningDebug(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping long-running debug test")
	}

	bm, _ := NewBLEManager("TestDevice")
	defer bm.Stop()

	t.Log("Starting 5 minute debug session")
	t.Log("Use BLE scanner to connect and test characteristics")

	err := bm.AcceptConnections(5 * time.Minute)
	if err != nil {
		t.Logf("Error during debug session: %v", err)
		t.FailNow()
	}

	// Wait for full duration
	time.Sleep(5 * time.Minute)
}

func TestExample(t *testing.T) {
	var adapter = bluetooth.DefaultAdapter
	// Enable BLE interface.
	must("enable BLE stack", adapter.Enable())

	// Define the peripheral device info.
	adv := adapter.DefaultAdvertisement()
	must("config adv", adv.Configure(bluetooth.AdvertisementOptions{
		LocalName: "Go Bluetooth",
	}))

	// Start advertising
	must("start adv", adv.Start())
	println("advertising...")
	for {
		// Sleep forever.
		time.Sleep(time.Hour)
	}
}

func must(action string, err error) {
	if err != nil {
		panic("failed to " + action + ": " + err.Error())
	}
}
