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

func TestRiskControlMerchantTableNameMatchesSecurityModel(t *testing.T) {
	t.Parallel()
	if got, want := (riskControlMerchantModel{}).TableName(), (security.RiskControlMerchantModel{}).TableName(); got != want {
		t.Fatalf("risk merchant table mismatch: got %s want %s", got, want)
	}
}

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

	if err := db.Exec(`DELETE FROM cc_sys_phone_attribution`).Error; err != nil {
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

func TestRuntimeSelectionMarkerNearbyCities(t *testing.T) {
	t.Parallel()

	db := openMarkerTestDB(t)
	// We need to create cc_sys_config table and insert the configuration
	if err := db.Exec(`CREATE TABLE IF NOT EXISTS cc_sys_config (config_key TEXT, config_value TEXT, config_desc TEXT)`).Error; err != nil {
		t.Fatal(err)
	}

	// Seed configuration
	if err := db.Exec(`INSERT INTO cc_sys_config (config_key, config_value, config_desc) VALUES (?, ?, ?)`,
		"system.nearby_cities", `{"510100":["511300","510600","510700"],"440100":["440600","441900","440300"]}`, "Nearby cities config").Error; err != nil {
		t.Fatal(err)
	}

	// Seed callee phone attribution
	if err := db.Create(&phoneAttributionModel{AreaCode: "1808000", Province: "四川省", City: "成都市", ProvCode: "510000", CityCode: "510100"}).Error; err != nil {
		t.Fatal(err)
	}

	// Seed candidate phone attributions to verify backfilling and code-based matching
	if err := db.Create(&phoneAttributionModel{AreaCode: "1801111", Province: "四川省", City: "德阳市", ProvCode: "510000", CityCode: "510600"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&phoneAttributionModel{AreaCode: "1801112", Province: "四川省", City: "成都市", ProvCode: "510000", CityCode: "510100"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&phoneAttributionModel{AreaCode: "1801113", Province: "四川省", City: "南充市", ProvCode: "510000", CityCode: "511300"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&phoneAttributionModel{AreaCode: "1801114", Province: "广东省", City: "广州市", ProvCode: "440000", CityCode: "440100"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&phoneAttributionModel{AreaCode: "1801115", Province: "四川省", City: "自贡市", ProvCode: "510000", CityCode: "510300"}).Error; err != nil {
		t.Fatal(err)
	}

	marker := &RuntimeSelectionMarker{DB: db}

	// Candidates list (leaving city/province empty to test backfilling)
	candidates := []cti.NumberCandidate{
		{Phone: "18011110001", Available: true, RiskAllowed: true, Concurrency: 1},
		{Phone: "18011120002", Available: true, RiskAllowed: true, Concurrency: 1},
		{Phone: "18011130003", Available: true, RiskAllowed: true, Concurrency: 1},
		{Phone: "18011140004", Available: true, RiskAllowed: true, Concurrency: 1},
		{Phone: "18011150005", Available: true, RiskAllowed: true, Concurrency: 1},
	}

	marked, err := marker.MarkCandidates(context.Background(), cti.SelectionRequest{
		MerchantID: "88",
		Callee:     "18080001234",
	}, candidates)
	if err != nil {
		t.Fatal(err)
	}

	if len(marked) != 5 {
		t.Fatalf("expected 5 marked candidates, got %d", len(marked))
	}

	// Verify backfilled text values
	if marked[1].City != "成都市" || marked[1].Province != "四川省" {
		t.Errorf("expected backfilled成都, got city=%s province=%s", marked[1].City, marked[1].Province)
	}

	expectedRanks := map[string]int{
		"18011110001": 3,    // 德阳 -> 2 + 1 = 3
		"18011120002": 1,    // 成都 -> local -> 1
		"18011130003": 2,    // 南充 -> 2 + 0 = 2
		"18011140004": 9999, // 广州 -> other province -> 9999
		"18011150005": 100,  // 自贡 -> same province -> 100
	}

	for _, cand := range marked {
		expected, exists := expectedRanks[cand.Phone]
		if !exists {
			t.Errorf("unexpected phone number: %s", cand.Phone)
			continue
		}
		if cand.LocalMatchRank != expected {
			t.Errorf("Phone %s: expected LocalMatchRank %d, got %d", cand.Phone, expected, cand.LocalMatchRank)
		}
	}

	// Now let's test sorting using Selector{}.Select
	result := cti.Selector{}.Select(context.Background(), cti.SelectionRequest{
		CallID:            "call-2",
		MerchantID:        "88",
		SelectionStrategy: "CONCURRENCY",
		Candidates:        marked,
	})

	if !result.Success || result.Caller == nil {
		t.Fatalf("expected selection to succeed, got %+v", result)
	}

	// The preferred one should be 成都市 (18011120002, Rank 1)
	if result.Caller.Phone != "18011120002" {
		t.Errorf("expected chosen phone to be 18011120002 (成都), got %s", result.Caller.Phone)
	}

	// If Chengdu number is not available, Nanchong (18011130003, Rank 2) should be selected.
	marked[1].Available = false
	result2 := cti.Selector{}.Select(context.Background(), cti.SelectionRequest{
		CallID:            "call-3",
		MerchantID:        "88",
		SelectionStrategy: "CONCURRENCY",
		Candidates:        marked,
	})
	if !result2.Success || result2.Caller == nil {
		t.Fatalf("expected selection to succeed, got %+v", result2)
	}
	if result2.Caller.Phone != "18011130003" {
		t.Errorf("expected chosen phone to be 18011130003 (南充), got %s", result2.Caller.Phone)
	}

	// If both Chengdu and Nanchong are unavailable, Deyang (18011110001, Rank 3) should be selected.
	marked[2].Available = false
	result3 := cti.Selector{}.Select(context.Background(), cti.SelectionRequest{
		CallID:            "call-4",
		MerchantID:        "88",
		SelectionStrategy: "CONCURRENCY",
		Candidates:        marked,
	})
	if !result3.Success || result3.Caller == nil {
		t.Fatalf("expected selection to succeed, got %+v", result3)
	}
	if result3.Caller.Phone != "18011110001" {
		t.Errorf("expected chosen phone to be 18011110001 (德阳), got %s", result3.Caller.Phone)
	}

	// If all local/nearby are unavailable, Zigong (18011150005, Rank 100) should be selected.
	marked[0].Available = false
	result4 := cti.Selector{}.Select(context.Background(), cti.SelectionRequest{
		CallID:            "call-5",
		MerchantID:        "88",
		SelectionStrategy: "CONCURRENCY",
		Candidates:        marked,
	})
	if !result4.Success || result4.Caller == nil {
		t.Fatalf("expected selection to succeed, got %+v", result4)
	}
	if result4.Caller.Phone != "18011150005" {
		t.Errorf("expected chosen phone to be 18011150005 (自贡), got %s", result4.Caller.Phone)
	}
}

var _ = contracts.CodeOK
