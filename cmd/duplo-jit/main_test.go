package main

import "testing"

// TestArgsContainFlag_HelpdeskInference guards the "subcommand name is optional" behavior: a
// kubeconfig's exec.args can omit "helpdesk-k8s" entirely as long as --workspace-id is present
// somewhere among the remaining flags.
func TestArgsContainFlag_HelpdeskInference(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want bool
	}{
		{"bare flag present", []string{"--host", "https://x", "--workspace-id", "ws-1"}, true},
		{"equals-form present", []string{"--host", "https://x", "--workspace-id=ws-1"}, true},
		{"absent", []string{"--host", "https://x", "--cluster-id", "c-1"}, false},
		{"empty args", []string{}, false},
		{"prefix collision must not match", []string{"--workspace-id-extra", "ws-1"}, false},
		{"value-only collision must not match", []string{"ws-1", "--workspace-id-extra"}, false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := argsContainFlag(c.args, "workspace-id")
			if got != c.want {
				t.Errorf("argsContainFlag(%v, %q) = %v, want %v", c.args, "workspace-id", got, c.want)
			}
		})
	}
}
