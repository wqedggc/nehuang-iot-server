package domain

import "testing"

func TestControlResultOrdering(t *testing.T) {
	tests := []struct {
		name    string
		initial CommandStatus
		results []CommandStatus
		want    CommandStatus
	}{
		{"ack then complete", CommandStatusSent, []CommandStatus{CommandStatusAck, CommandStatusSucceeded}, CommandStatusSucceeded},
		{"late ack", CommandStatusSent, []CommandStatus{CommandStatusSucceeded, CommandStatusAck}, CommandStatusSucceeded},
		{"duplicate ack", CommandStatusSent, []CommandStatus{CommandStatusAck, CommandStatusAck}, CommandStatusAck},
		{"duplicate terminal", CommandStatusSent, []CommandStatus{CommandStatusFailed, CommandStatusFailed}, CommandStatusFailed},
		{"timeout then late success", CommandStatusSent, []CommandStatus{CommandStatusTimeout, CommandStatusSucceeded}, CommandStatusTimeout},
		{"conflicting terminal", CommandStatusSent, []CommandStatus{CommandStatusSucceeded, CommandStatusFailed}, CommandStatusSucceeded},
		{"result before sent callback", CommandStatusCreated, []CommandStatus{CommandStatusSucceeded}, CommandStatusSucceeded},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := tt.initial
			for _, next := range tt.results {
				if state.CanAdvanceResult(next) {
					state = next
				}
			}
			if state != tt.want {
				t.Fatalf("got %v want %v", state, tt.want)
			}
		})
	}
	for _, terminal := range []CommandStatus{CommandStatusSucceeded, CommandStatusFailed, CommandStatusTimeout} {
		if !terminal.IsTerminal() {
			t.Fatal("terminal status unrecognized")
		}
		for _, next := range []CommandStatus{CommandStatusAck, CommandStatusSucceeded, CommandStatusFailed, CommandStatusTimeout} {
			if terminal.CanAdvanceResult(next) {
				t.Fatal("terminal regressed")
			}
		}
	}
	if CommandStatusAck.IsTerminal() || CommandStatusSent.CanAdvanceResult(CommandStatusCreated) {
		t.Fatal("invalid lifecycle")
	}
}
