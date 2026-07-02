package devices

import "testing"

func TestPickLeastLoaded(t *testing.T) {
	newDevices := func() []Device {
		return []Device{
			{DeviceInput: DeviceInput{ID: "a"}},
			{DeviceInput: DeviceInput{ID: "b"}},
			{DeviceInput: DeviceInput{ID: "c"}},
		}
	}

	t.Run("picks the device with the fewest pending messages", func(t *testing.T) {
		counts := map[string]int{"a": 5, "b": 1, "c": 3}

		got := pickLeastLoaded(newDevices(), counts)
		if got.ID != "b" {
			t.Fatalf("expected device b, got %s", got.ID)
		}
	})

	t.Run("treats missing devices as zero load", func(t *testing.T) {
		counts := map[string]int{"a": 5, "b": 3} // "c" absent -> 0

		got := pickLeastLoaded(newDevices(), counts)
		if got.ID != "c" {
			t.Fatalf("expected device c, got %s", got.ID)
		}
	})

	t.Run("single device is returned as-is", func(t *testing.T) {
		got := pickLeastLoaded([]Device{{DeviceInput: DeviceInput{ID: "only"}}}, map[string]int{"only": 99})
		if got.ID != "only" {
			t.Fatalf("expected device only, got %s", got.ID)
		}
	})

	t.Run("ties resolve to one of the minima", func(t *testing.T) {
		counts := map[string]int{"a": 2, "b": 2, "c": 7}

		for i := 0; i < 50; i++ {
			got := pickLeastLoaded(newDevices(), counts)
			if got.ID == "c" {
				t.Fatalf("iteration %d: picked overloaded device c", i)
			}
		}
	})
}
