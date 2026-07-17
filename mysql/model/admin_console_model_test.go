package mysqlmodel

import "testing"

func TestMergeAdminDailyFeatureRecordsSortsAndPaginatesNewestFirst(t *testing.T) {
	query := AdminPageQuery{Page: 1, PageSize: 2}
	firstPage, total, err := mergeAdminDailyFeatureRecords(
		query,
		[]adminDailyUIDCount{{Date: "2026-07-14", UserCount: 2}, {Date: "2026-07-16", UserCount: 4}},
		[]adminDailyUIDCount{{Date: "2026-07-16", UserCount: 3}},
		[]adminDailyUIDCount{{Date: "2026-07-15", UserCount: 5}},
		[]adminDailyUIDCount{{Date: "2026-07-14", UserCount: 1}},
	)
	if err != nil {
		t.Fatal(err)
	}
	if total != 3 {
		t.Fatalf("total = %d, want 3", total)
	}
	if len(firstPage) != 2 || firstPage[0].Date != "2026-07-16" || firstPage[1].Date != "2026-07-15" {
		t.Fatalf("first page = %#v, want dates 2026-07-16 and 2026-07-15", firstPage)
	}
	if firstPage[0].WeightUsers != 4 || firstPage[0].TrainingTagUsers != 3 {
		t.Fatalf("merged newest day = %#v", firstPage[0])
	}

	query.Page = 2
	secondPage, total, err := mergeAdminDailyFeatureRecords(
		query,
		[]adminDailyUIDCount{{Date: "2026-07-14", UserCount: 2}, {Date: "2026-07-16", UserCount: 4}},
		[]adminDailyUIDCount{{Date: "2026-07-16", UserCount: 3}},
		[]adminDailyUIDCount{{Date: "2026-07-15", UserCount: 5}},
		[]adminDailyUIDCount{{Date: "2026-07-14", UserCount: 1}},
	)
	if err != nil {
		t.Fatal(err)
	}
	if total != 3 || len(secondPage) != 1 || secondPage[0].Date != "2026-07-14" {
		t.Fatalf("second page = %#v, total = %d", secondPage, total)
	}
	if secondPage[0].WeightUsers != 2 || secondPage[0].BodyPhotoUsers != 1 {
		t.Fatalf("merged oldest day = %#v", secondPage[0])
	}
}

func TestMergeAdminDailyFeatureRecordsReturnsEmptyArray(t *testing.T) {
	items, total, err := mergeAdminDailyFeatureRecords(AdminPageQuery{Page: 1, PageSize: 30}, nil, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if total != 0 || items == nil || len(items) != 0 {
		t.Fatalf("items = %#v, total = %d, want non-nil empty list", items, total)
	}
}
