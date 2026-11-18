package recorder

import "testing"

func TestProcessOwnershipMatching(t *testing.T) {
	owned := []ProcessIdentity{
		{PID: 4200, CreationTime: 9001, SessionToken: "sess-abc", Name: "cs2.exe"},
	}

	// Exact match -> not external.
	if HasExternalProcess([]ProcessIdentity{
		{PID: 4200, CreationTime: 9001, SessionToken: "-con_logfile _strikeye_session_sess-abc.log", Name: "cs2.exe"},
	}, "cs2.exe", owned) {
		t.Fatalf("expected owned cs2.exe not to be flagged as external")
	}

	// Different PID -> external user CS2.
	if !HasExternalProcess([]ProcessIdentity{
		{PID: 4200, CreationTime: 9001, SessionToken: "sess-abc", Name: "cs2.exe"},
		{PID: 9999, CreationTime: 9002, Name: "cs2.exe"},
	}, "cs2.exe", owned) {
		t.Fatalf("expected second cs2.exe to be detected as external")
	}

	// Same PID but different CreationTime (PID reuse) -> external.
	if !HasExternalProcess([]ProcessIdentity{
		{PID: 4200, CreationTime: 99999, Name: "cs2.exe"},
	}, "cs2.exe", owned) {
		t.Fatalf("expected PID-reused cs2.exe with mismatched CreationTime to be external")
	}
}
