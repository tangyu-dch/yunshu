package callflow

import (
	"time"

	"yunshu/internal/domain/outbox"
)

const DestinationBillingSettlement = "cti_billing_settlement"

// BuildBillingSettlementEntry 根据已估算的计费流水构造结算节点。
func BuildBillingSettlementEntry(source outbox.Entry, now time.Time) outbox.Entry {
	payload := make(map[string]any, len(source.Payload)+4)
	for key, value := range source.Payload {
		payload[key] = value
	}
	payload["sourceOutboxId"] = source.ID
	payload["eventType"] = "billing_rated"
	return outbox.Entry{
		ID:             "billing:settlement:" + source.AggregateID,
		AggregateType:  "call_billing_ledger",
		AggregateID:    source.AggregateID,
		Destination:    DestinationBillingSettlement,
		IdempotencyKey: "billing:settlement:" + source.AggregateID,
		Payload:        payload,
		NextAttemptAt:  now.UTC(),
	}
}
