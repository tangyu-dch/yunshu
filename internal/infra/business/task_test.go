package business

import (
	"testing"
)

func TestBatchModelsMapTables(t *testing.T) {
	t.Parallel()

	if (MerchantBatchCallTaskModel{}).TableName() != "cc_biz_task" {
		t.Fatalf("unexpected batch task table")
	}
	if (MerchantBatchCallTaskListModel{}).TableName() != "cc_biz_task_list" {
		t.Fatalf("unexpected batch task list table")
	}
}
