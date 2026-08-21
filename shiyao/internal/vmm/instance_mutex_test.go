package vmm

import "testing"

func TestInstanceLifecycleStateTransitionsRemainSerialized(t *testing.T) {
	states := []State{StateCreated, StateConfiguring, StateConfigured, StateRunning, StateStopping, StateStopped}
	for idx := 1; idx < len(states); idx++ {
		if states[idx] == states[idx-1] {
			t.Fatalf("duplicate lifecycle state at index %d: %s", idx, states[idx])
		}
	}
}
