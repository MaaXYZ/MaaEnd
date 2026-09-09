package listcomplete

import "testing"

func TestAdvanceUnchangedRequiresConsecutiveMatches(t *testing.T) {
	hits, complete := advanceUnchanged(0, true, 2)
	if hits != 1 || complete {
		t.Fatalf("first unchanged frame = (%d, %t), want (1, false)", hits, complete)
	}

	hits, complete = advanceUnchanged(hits, false, 2)
	if hits != 0 || complete {
		t.Fatalf("changed frame = (%d, %t), want (0, false)", hits, complete)
	}

	hits, complete = advanceUnchanged(hits, true, 2)
	if hits != 1 || complete {
		t.Fatalf("first unchanged frame after reset = (%d, %t), want (1, false)", hits, complete)
	}

	hits, complete = advanceUnchanged(hits, true, 2)
	if hits != 2 || !complete {
		t.Fatalf("second consecutive unchanged frame = (%d, %t), want (2, true)", hits, complete)
	}
}

func TestParseParamsPreservesDefaultBehavior(t *testing.T) {
	p, err := parseParams("")
	if err != nil {
		t.Fatal(err)
	}
	if p.UnchangedRequired != 1 {
		t.Fatalf("default unchanged_required = %d, want 1", p.UnchangedRequired)
	}

	p, err = parseParams(`{"unchanged_required":2}`)
	if err != nil {
		t.Fatal(err)
	}
	if p.UnchangedRequired != 2 {
		t.Fatalf("configured unchanged_required = %d, want 2", p.UnchangedRequired)
	}
}
