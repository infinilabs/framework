package task

import "testing"

func TestGetScheduleOptionsWithInitialDelay(t *testing.T) {
	task := &ScheduleTask{
		Type:         Interval,
		InitialDelay: "250ms",
	}

	options := getScheduleOptions(task)
	if len(options) != 1 {
		t.Fatalf("expected one schedule option, got %d", len(options))
	}
}

func TestGetScheduleOptionsSkipsInvalidDelay(t *testing.T) {
	task := &ScheduleTask{
		Type:         Interval,
		InitialDelay: "invalid",
	}

	options := getScheduleOptions(task)
	if len(options) != 0 {
		t.Fatalf("expected no schedule options for invalid delay, got %d", len(options))
	}
}
