package domain

import "testing"

func TestCommandStatusLabel(t *testing.T) {
	tests := []struct {
		status CommandStatus
		want   string
	}{
		{CommandStatusCreated, "pending"},
		{CommandStatusSent, "sent"},
		{CommandStatusAck, "ack"},
		{CommandStatusSucceeded, "succeeded"},
		{CommandStatusFailed, "failed"},
		{CommandStatusTimeout, "timeout"},
	}
	for _, tt := range tests {
		if got := tt.status.Label(); got != tt.want {
			t.Fatalf("CommandStatus(%d).Label() = %q, want %q", tt.status, got, tt.want)
		}
	}
}
