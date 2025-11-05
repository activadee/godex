package codexexec

import (
	"slices"
	"testing"
)

func TestBuildCommandArgsConfigOverridesWithoutProfile(t *testing.T) {
	commandArgs := buildCommandArgs(Args{
		ConfigOverrides: map[string]any{
			"beta":   true,
			"alpha":  42,
			"empty":  "",
			"spaces": "  ",
		},
	})

	var configs []string
	for i := 0; i < len(commandArgs); i++ {
		switch commandArgs[i] {
		case "--profile":
			t.Fatalf("unexpected --profile flag: %+v", commandArgs)
		case "-c":
			i++
			if i >= len(commandArgs) {
				t.Fatalf("missing value after -c in %v", commandArgs)
			}
			configs = append(configs, commandArgs[i])
		}
	}

	expected := []string{"alpha=42", "beta=true", "empty=", "spaces=  "}
	if !slices.Equal(configs, expected) {
		t.Fatalf("expected configs %v, got %v", expected, configs)
	}
}

func TestBuildCommandArgsConfigOverridesWithProfile(t *testing.T) {
	commandArgs := buildCommandArgs(Args{
		ConfigOverrides: map[string]any{
			"profile": "dev",
			"alpha":   1,
		},
	})

	var (
		configs      []string
		profileCount int
	)

	for i := 0; i < len(commandArgs); i++ {
		switch commandArgs[i] {
		case "--profile":
			profileCount++
			if profileCount > 1 {
				t.Fatalf("profile flag emitted more than once: %v", commandArgs)
			}
			i++
			if i >= len(commandArgs) {
				t.Fatalf("missing value after --profile in %v", commandArgs)
			}
			if got := commandArgs[i]; got != "dev" {
				t.Fatalf("expected profile value dev, got %q", got)
			}
		case "-c":
			i++
			if i >= len(commandArgs) {
				t.Fatalf("missing value after -c in %v", commandArgs)
			}
			configs = append(configs, commandArgs[i])
		}
	}

	if profileCount != 1 {
		t.Fatalf("expected --profile flag once, saw %d times", profileCount)
	}

	expected := []string{"alpha=1"}
	if !slices.Equal(configs, expected) {
		t.Fatalf("expected configs %v, got %v", expected, configs)
	}
}

func TestBuildCommandArgsConfigOverridesDeterministicOrder(t *testing.T) {
	configs := map[string]any{
		"beta":  2,
		"gamma": 3,
		"alpha": 1,
	}

	var expected []string
	for i := 0; i < 25; i++ {
		commandArgs := buildCommandArgs(Args{ConfigOverrides: configs})

		var collected []string
		for idx := 0; idx < len(commandArgs); idx++ {
			if commandArgs[idx] == "-c" {
				idx++
				if idx >= len(commandArgs) {
					t.Fatalf("missing value after -c in %v", commandArgs)
				}
				collected = append(collected, commandArgs[idx])
			}
		}

		if expected == nil {
			expected = collected
			continue
		}
		if !slices.Equal(collected, expected) {
			t.Fatalf("expected order %v on iteration %d, got %v", expected, i, collected)
		}
	}

	want := []string{"alpha=1", "beta=2", "gamma=3"}
	if !slices.Equal(expected, want) {
		t.Fatalf("expected configs %v, got %v", want, expected)
	}
}
