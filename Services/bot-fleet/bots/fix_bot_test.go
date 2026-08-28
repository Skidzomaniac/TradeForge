package bots

import (
	"strings"
	"testing"
)

func TestBuildFIXMessage_HasRequiredTags(t *testing.T) {
	b := NewFIXBot("localhost:5001", "BOT", "EXCH")
	msg := b.buildFIXMessage("D", "11=ord1"+SOH+"54=1"+SOH+"38=10")
	for _, tag := range []string{"8=FIX.4.2", "9=", "35=D", "49=BOT", "56=EXCH", "34=1", "10="} {
		if !strings.Contains(msg, tag) {
			t.Errorf("message missing %q: %q", tag, strings.ReplaceAll(msg, SOH, "|"))
		}
	}
}

func TestChecksum_Deterministic(t *testing.T) {
	if checksum("abc") != checksum("abc") {
		t.Fatal("checksum not deterministic")
	}
	if len(checksum("anything")) != 3 {
		t.Fatal("checksum must be 3 digits")
	}
}

func TestParseFIXMessage(t *testing.T) {
	raw := "8=FIX.4.2" + SOH + "35=8" + SOH + "39=2" + SOH
	m := parseFIXMessage(raw)
	if m["35"] != "8" || m["39"] != "2" {
		t.Fatalf("parse failed: %+v", m)
	}
}

func TestSeqNumIncrements(t *testing.T) {
	b := NewFIXBot("x", "S", "T")
	m1 := b.buildFIXMessage("D", "")
	m2 := b.buildFIXMessage("D", "")
	if !strings.Contains(m1, "34=1"+SOH) || !strings.Contains(m2, "34=2"+SOH) {
		t.Fatal("sequence number should increment")
	}
}
