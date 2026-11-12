package models

import "testing"

func TestContainerRuntimeString(t *testing.T) {
	tests := []struct {
		runtime ContainerRuntime
		want    string
	}{
		{runtime: Podman, want: "Podman"},
		{runtime: Docker, want: "Docker"},
		{runtime: Both, want: "Both"},
		{runtime: ContainerRuntime(99), want: "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.runtime.String(); got != tt.want {
				t.Fatalf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestScenarioNodeGetParent(t *testing.T) {
	parent := "root"
	node := ScenarioNode{Parent: &parent}
	if got := node.GetParent(); got != &parent || *got != parent {
		t.Fatalf("GetParent() = %v, want %q", got, parent)
	}

	if got := (ScenarioNode{}).GetParent(); got != nil {
		t.Fatalf("GetParent() = %v, want nil", got)
	}
}
