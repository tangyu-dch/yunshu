package telephony

import (
	"testing"
	"time"
)

func TestGormModelMapsFreeswitchTable(t *testing.T) {
	t.Parallel()

	model := FreeswitchModel{
		ID:           9,
		Address:      "10.0.0.9",
		LocalAddress: "172.16.0.9",
		ESLPort:      8021,
		SIPPort:      5060,
		Password:     "ClueCon",
		SetID:        2,
		Weight:       80,
		RWeight:      60,
		CC:           1,
		CmdPort:      9090,
		Canary:       true,
		Enable:       true,
		UpdatedTime:  time.Date(2026, 5, 22, 10, 0, 0, 0, time.UTC),
	}

	node := nodeFromModel(model)
	if node.FSAddr != "10.0.0.9:8021" {
		t.Fatalf("unexpected fs addr: %s", node.FSAddr)
	}
	if node.CommandURL != "10.0.0.9:9090" {
		t.Fatalf("unexpected command url: %s", node.CommandURL)
	}
	if node.SetID != 2 || node.Weight != 80 || node.RWeight != 60 || !node.Canary {
		t.Fatalf("unexpected node mapping: %+v", node)
	}
	if (FreeswitchModel{}).TableName() != "cc_tel_freeswitch" {
		t.Fatalf("unexpected table name")
	}
}

func TestSplitFSAddr(t *testing.T) {
	t.Parallel()

	address, port, ok := splitFSAddr("10.0.0.9:8021")
	if !ok || address != "10.0.0.9" || port != 8021 {
		t.Fatalf("unexpected split: %s %d %v", address, port, ok)
	}
}
