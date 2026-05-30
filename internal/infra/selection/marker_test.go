package selection

import (
	"context"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"yunshu/internal/contracts"
	"yunshu/internal/domain/cti"
	"yunshu/internal/infra/security"
)

func TestRuntimeSelectionMarkerAppliesWhitelistAndBlacklist(t *testing.T) {
	t.Parallel()

	db := openMarkerTestDB(t)
	marker := &RuntimeSelectionMarker{DB: db}

	if err := db.Create(&security.WhitelistDataModel{ID: 1, Phone: "13800000000", NumberType: "CALLEE", Enable: true, DelFlag: false}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&security.WhitelistDataMerchantModel{WhiteID: 1, MerchantID: 88}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&security.BlacklistModel{ID: 7, Name: "系统黑名单", VerificationChannel: 1, Enable: true, DelFlag: false}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&security.BlacklistGatewayModel{BlacklistID: 7, GatewayID: 10}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO cc_sec_blacklist_data(phone, black_level, remark, created_time, updated_time) VALUES (?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`, "13900000000", "LEVEL_1", "命中黑名单").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&riskControlModel{ID: 5, BlackLevelFlag: true, BlackLevel: "LEVEL_2", DelFlag: false}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&riskControlMerchantModel{RiskID: 5, MerchantID: 88, Enable: true}).Error; err != nil {
		t.Fatal(err)
	}

	marked, err := marker.MarkCandidates(context.Background(), cti.SelectionRequest{
		MerchantID: "88",
		Callee:     "13800000000",
		Candidates: []cti.NumberCandidate{
			{Phone: "1001", GatewayID: "10", Available: true, RiskAllowed: true, Concurrency: 1},
			{Phone: "1002", GatewayID: "11", Available: true, RiskAllowed: true, Concurrency: 1},
		},
	}, []cti.NumberCandidate{
		{Phone: "1001", GatewayID: "10", Available: true, RiskAllowed: true, Concurrency: 1},
		{Phone: "1002", GatewayID: "11", Available: true, RiskAllowed: true, Concurrency: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !marked[0].WhitelistHit || marked[0].BlacklistHit {
		t.Fatalf("expected callee whitelist to win, got %+v", marked[0])
	}
	if !marked[1].WhitelistHit || marked[1].BlacklistHit {
		t.Fatalf("expected callee whitelist to apply to all candidates, got %+v", marked[1])
	}

	marked, err = marker.MarkCandidates(context.Background(), cti.SelectionRequest{
		MerchantID: "88",
		Callee:     "13900000000",
	}, []cti.NumberCandidate{
		{Phone: "1001", GatewayID: "10", Available: true, RiskAllowed: true, Concurrency: 1},
		{Phone: "1002", GatewayID: "11", Available: true, RiskAllowed: true, Concurrency: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !marked[0].BlacklistHit || marked[0].WhitelistHit {
		t.Fatalf("expected gateway 10 blacklisted, got %+v", marked[0])
	}
	if !marked[1].BlacklistHit {
		t.Fatalf("expected merchant risk blacklist to block all candidates, got %+v", marked[1])
	}
}

func TestSelectorPrefersWhitelistHit(t *testing.T) {
	t.Parallel()

	result := cti.Selector{}.Select(context.Background(), cti.SelectionRequest{
		CallID: "call-1",
		Candidates: []cti.NumberCandidate{
			{Phone: "1001", Available: true, RiskAllowed: true, Concurrency: 1, BlacklistHit: true},
			{Phone: "1002", Available: true, RiskAllowed: true, Concurrency: 1, WhitelistHit: true},
		},
	})
	if !result.Success || result.Caller == nil || result.Caller.Phone != "1002" {
		t.Fatalf("expected whitelist candidate selected, got %+v", result)
	}
}

func TestRuntimeSelectionMarkerAppliesBlindSpotAndFrequency(t *testing.T) {
	t.Parallel()

	db := openMarkerTestDB(t)
	marker := &RuntimeSelectionMarker{DB: db}

	if err := db.Create(&phoneAttributionModel{AreaCode: "1380013", CityCode: "310100"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&riskControlModel{
		ID:                  9,
		BlindAreaFlag:       true,
		BlindArea:           "310100,320100",
		CalleeFrequencyFlag: true,
		CalleeFrequency:     `[{"day":2,"count":3,"type":1}]`,
		DelFlag:             false,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&riskControlMerchantModel{RiskID: 9, MerchantID: 99, Enable: true}).Error; err != nil {
		t.Fatal(err)
	}

	marked, err := marker.MarkCandidates(context.Background(), cti.SelectionRequest{
		MerchantID: "99",
		Callee:     "13800138000",
	}, []cti.NumberCandidate{{Phone: "1001", GatewayID: "10", Available: true, RiskAllowed: true, Concurrency: 1}})
	if err != nil {
		t.Fatal(err)
	}
	if marked[0].RiskAllowed {
		t.Fatalf("expected blind spot to block candidate, got %+v", marked[0])
	}

	if err := db.Exec(`DELETE FROM cc_sys_attribution`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&calleeFeatureModel{
		CalledNumber:     "13900139000",
		MerchantID:       99,
		ChannelID:        "1",
		StatDate:         time.Now().UTC(),
		CallDialCount:    3,
		CallConnectCount: 1,
	}).Error; err != nil {
		t.Fatal(err)
	}

	marked, err = marker.MarkCandidates(context.Background(), cti.SelectionRequest{
		MerchantID: "99",
		Callee:     "13900139000",
	}, []cti.NumberCandidate{{Phone: "1001", GatewayID: "10", Available: true, RiskAllowed: true, Concurrency: 1}})
	if err != nil {
		t.Fatal(err)
	}
	if marked[0].RiskAllowed {
		t.Fatalf("expected callee frequency to block candidate, got %+v", marked[0])
	}
}

func TestRuntimeSelectionMarkerAppliesChannelBlindSpotAndFrequency(t *testing.T) {
	t.Parallel()

	db := openMarkerTestDB(t)
	marker := &RuntimeSelectionMarker{DB: db}

	if err := db.Exec(`CREATE TABLE cc_tel_channel (id integer primary key, config text, blind_area text, enable boolean, del_flag boolean)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&phoneAttributionModel{AreaCode: "1380013", ProvCode: "310000", CityCode: "310100"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO cc_tel_channel(id, config, blind_area, enable, del_flag) VALUES (1, ?, ?, 1, 0)`, `[{"day":2,"count":3}]`, "310100").Error; err != nil {
		t.Fatal(err)
	}

	marked, err := marker.MarkCandidates(context.Background(), cti.SelectionRequest{
		MerchantID: "66",
		Callee:     "13800138000",
	}, []cti.NumberCandidate{{Phone: "1001", ChannelID: 1, Available: true, RiskAllowed: true, Concurrency: 1}})
	if err != nil {
		t.Fatal(err)
	}
	if marked[0].RiskAllowed {
		t.Fatalf("expected channel blind area to block candidate, got %+v", marked[0])
	}

	if err := db.Exec(`UPDATE cc_tel_channel SET blind_area = '' WHERE id = 1`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&calleeFeatureModel{
		CalledNumber:     "13900139000",
		MerchantID:       66,
		ChannelID:        "1",
		StatDate:         time.Now().UTC(),
		CallDialCount:    3,
		CallConnectCount: 0,
	}).Error; err != nil {
		t.Fatal(err)
	}
	marked, err = marker.MarkCandidates(context.Background(), cti.SelectionRequest{
		MerchantID: "66",
		Callee:     "13900139000",
	}, []cti.NumberCandidate{{Phone: "1001", ChannelID: 1, Available: true, RiskAllowed: true, Concurrency: 1}})
	if err != nil {
		t.Fatal(err)
	}
	if marked[0].RiskAllowed {
		t.Fatalf("expected channel frequency to block candidate, got %+v", marked[0])
	}

	marked, err = marker.MarkCandidates(context.Background(), cti.SelectionRequest{
		MerchantID: "66",
		Callee:     "13900139000",
	}, []cti.NumberCandidate{{Phone: "1001", ChannelID: 1, Available: true, RiskAllowed: true, Concurrency: 1, WhitelistHit: true}})
	if err != nil {
		t.Fatal(err)
	}
	if !marked[0].RiskAllowed {
		t.Fatalf("expected whitelist hit to bypass channel risk filters, got %+v", marked[0])
	}
}

func openMarkerTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&security.WhitelistDataModel{}, &security.WhitelistDataMerchantModel{}, &security.BlacklistModel{}, &security.BlacklistGatewayModel{}, &riskControlModel{}, &riskControlMerchantModel{}); err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&phoneAttributionModel{}, &calleeFeatureModel{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE IF NOT EXISTS cc_sec_blacklist_data (phone TEXT, black_level TEXT, remark TEXT, created_time datetime, updated_time datetime)`).Error; err != nil {
		t.Fatal(err)
	}
	return db
}

var _ = contracts.CodeOK
