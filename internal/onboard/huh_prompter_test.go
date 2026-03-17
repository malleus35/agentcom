package onboard

import (
	"bytes"
	"testing"
)

func TestNewHuhPrompterRetainsConstructorValues(t *testing.T) {
	input := bytes.NewBufferString("")
	output := &bytes.Buffer{}
	prompter := NewHuhPrompter(true, input, output)

	if !prompter.accessible {
		t.Fatal("accessible = false, want true")
	}
	if prompter.input != input {
		t.Fatal("input reader was not retained")
	}
	if prompter.output != output {
		t.Fatal("output writer was not retained")
	}
}
