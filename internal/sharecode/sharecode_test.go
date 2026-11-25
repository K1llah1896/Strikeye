package sharecode

import "testing"

// Vectors from Valve match share codes (cross-checked against akiver/cs-demo-analyzer).
var vectors = []Match{
	{MatchID: 3400360672356205056, ReservationID: 3400367402569957763, TVPort: 9725, ShareCode: "CSGO-L9spZ-ihuov-cyhtE-kxbqa-FkBAA"},
	{MatchID: 3230642215713767580, ReservationID: 3230647599455273103, TVPort: 55788, ShareCode: "CSGO-GADqf-jjyJ8-cSP2r-smZRo-TO2xK"},
	{MatchID: 3325408798641750542, ReservationID: 3325410334092558852, TVPort: 240, ShareCode: "CSGO-bPQEz-PrYTq-u5w8E-ZbUy7-ZeQ3A"},
	{MatchID: 3302232779302895618, ReservationID: 3302241568953467250, TVPort: 3085, ShareCode: "CSGO-wBrm6-7fkM6-AzBC5-u6GmR-iHLHA"},
	{MatchID: 3402250361329680757, ReservationID: 3402250801563828781, TVPort: 61630, ShareCode: "CSGO-TKDTJ-YrAXs-sDNfL-HOuKO-i84VH"},
	{MatchID: 3402249502336221574, ReservationID: 3402252092201501292, TVPort: 14119, ShareCode: "CSGO-p4X9o-3Mfut-tpe5y-J8K6f-mj5ZJ"},
}

func TestEncodeMatchesValveVectors(t *testing.T) {
	for _, v := range vectors {
		if got := Encode(v.MatchID, v.ReservationID, v.TVPort); got != v.ShareCode {
			t.Errorf("Encode(%d) = %s, want %s", v.MatchID, got, v.ShareCode)
		}
	}
}

func TestDecodeMatchesValveVectors(t *testing.T) {
	for _, v := range vectors {
		got, err := Decode(v.ShareCode)
		if err != nil {
			t.Fatalf("Decode(%s): %v", v.ShareCode, err)
		}
		if got.MatchID != v.MatchID || got.ReservationID != v.ReservationID || got.TVPort != v.TVPort {
			t.Errorf("Decode(%s) = %+v, want %+v", v.ShareCode, got, v)
		}
		if got.ShareCode != v.ShareCode {
			t.Errorf("canonical = %s, want %s", got.ShareCode, v.ShareCode)
		}
	}
}

func TestDecodeAcceptsLooseInput(t *testing.T) {
	got, err := Decode("  L9spZ-ihuov-cyhtE-kxbqa-FkBAA  ")
	if err != nil {
		t.Fatalf("decode loose: %v", err)
	}
	if got.MatchID != vectors[0].MatchID || got.ShareCode != vectors[0].ShareCode {
		t.Fatalf("loose input decoded to %+v", got)
	}
}

func TestDecodeRejectsBadInput(t *testing.T) {
	cases := []string{"", "CSGO-", "CSGO-L9spZ-ihuov-cyhtE-kxbqa", "CSGO-L9spZ-ihuov-cyhtE-kxbqa-FkB0A"}
	for _, c := range cases {
		if _, err := Decode(c); err == nil {
			t.Errorf("expected error for %q", c)
		}
	}
}
