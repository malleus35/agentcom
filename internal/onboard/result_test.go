package onboard

import "testing"

func TestResultValidate(t *testing.T) {
	tests := []struct {
		name    string
		result  Result
		wantErr bool
	}{
		{name: "valid without template", result: Result{HomeDir: "/tmp/agentcom", Confirmed: true}},
		{name: "valid company template", result: Result{HomeDir: "/tmp/agentcom", Template: "company", Confirmed: true}},
		{name: "valid oh-my-opencode template", result: Result{HomeDir: "/tmp/agentcom", Template: "oh-my-opencode", Confirmed: true}},
		{name: "missing home", result: Result{Confirmed: true}, wantErr: true},
		{name: "relative home", result: Result{HomeDir: "relative/path", Confirmed: true}, wantErr: true},
		{name: "invalid template", result: Result{HomeDir: "/tmp/agentcom", Template: "missing", Confirmed: true}, wantErr: true},
		{name: "not confirmed", result: Result{HomeDir: "/tmp/agentcom"}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.result.Validate()
			if tt.wantErr && err == nil {
				t.Fatal("Validate() error = nil, want error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("Validate() error = %v", err)
			}
		})
	}
}
