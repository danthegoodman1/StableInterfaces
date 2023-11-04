package stableinterfaces

import (
	"testing"
)

func TestExpansionNotation(t *testing.T) {
	out, err := expandRangePattern("host-{0..1}")
	if err != nil {
		t.Fatal(err)
	}

	expected := []string{"host-0", "host-1"}
	if out[0] != expected[0] || out[1] != expected[1] {
		t.Log(out)
		t.Fatal("mismatched IDs")
	}

	out, err = expandRangePattern("host-{0..3}-abc")
	if err != nil {
		t.Fatal(err)
	}

	expected = []string{"host-0-abc", "host-1-abc", "host-2-abc", "host-3-abc"}
	if out[0] != expected[0] || out[1] != expected[1] || out[2] != expected[2] || out[3] != expected[3] {
		t.Log(out)
		t.Fatal("mismatched IDs")
	}
}
