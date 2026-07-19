package mysqlmodel

import (
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	pb "spider-server/gen/spider/api"
)

func TestAdminClientSyncFailurePaginationUsesStableNewestFirstOrder(t *testing.T) {
	db, err := gorm.Open(mysql.New(mysql.Config{
		DSN:                       "gorm:gorm@tcp(localhost:9910)/gorm?charset=utf8&parseTime=True&loc=Local",
		SkipInitializeWithVersion: true,
	}), &gorm.Config{DryRun: true, DisableAutomaticPing: true})
	if err != nil {
		t.Fatal(err)
	}

	query := AdminPageQuery{Page: 1, PageSize: 2}
	pageOneSQL := db.ToSQL(func(tx *gorm.DB) *gorm.DB {
		return applyAdminClientSyncFailureOrderingAndPage(tx.Table("client_sync_failures AS f"), query).
			Find(&[]AdminClientSyncFailureRecord{})
	})
	query.Page = 2
	pageTwoSQL := db.ToSQL(func(tx *gorm.DB) *gorm.DB {
		return applyAdminClientSyncFailureOrderingAndPage(tx.Table("client_sync_failures AS f"), query).
			Find(&[]AdminClientSyncFailureRecord{})
	})

	const stableOrder = "ORDER BY f.last_failed_at DESC, f.id DESC"
	if !strings.Contains(pageOneSQL, stableOrder) || !strings.Contains(pageTwoSQL, stableOrder) {
		t.Fatalf("pagination SQL must order equal timestamps by id: page 1 = %q, page 2 = %q", pageOneSQL, pageTwoSQL)
	}
	if !strings.Contains(pageOneSQL, "LIMIT 2") || strings.Contains(pageOneSQL, "OFFSET") {
		t.Fatalf("page 1 SQL = %q", pageOneSQL)
	}
	if !strings.Contains(pageTwoSQL, "LIMIT 2 OFFSET 2") {
		t.Fatalf("page 2 SQL = %q", pageTwoSQL)
	}
}

func TestAdminPaywallSessionPaginationUsesStableNewestFirstOrder(t *testing.T) {
	db, err := gorm.Open(mysql.New(mysql.Config{
		DSN:                       "gorm:gorm@tcp(localhost:9910)/gorm?charset=utf8&parseTime=True&loc=Local",
		SkipInitializeWithVersion: true,
	}), &gorm.Config{DryRun: true, DisableAutomaticPing: true})
	if err != nil {
		t.Fatal(err)
	}

	pageOneSQL := db.ToSQL(func(tx *gorm.DB) *gorm.DB {
		return applyAdminPaywallSessionOrderingAndPage(tx.Table("paywall_session_records AS p"), AdminPageQuery{Page: 1, PageSize: 2}).
			Find(&[]AdminPaywallSessionRecord{})
	})
	pageTwoSQL := db.ToSQL(func(tx *gorm.DB) *gorm.DB {
		return applyAdminPaywallSessionOrderingAndPage(tx.Table("paywall_session_records AS p"), AdminPageQuery{Page: 2, PageSize: 2}).
			Find(&[]AdminPaywallSessionRecord{})
	})

	const stableOrder = "ORDER BY p.presented_at DESC, p.id DESC"
	if !strings.Contains(pageOneSQL, stableOrder) || !strings.Contains(pageTwoSQL, stableOrder) {
		t.Fatalf("paywall pagination must use stable newest-first order: page 1 = %q, page 2 = %q", pageOneSQL, pageTwoSQL)
	}
	if !strings.Contains(pageOneSQL, "LIMIT 2") || strings.Contains(pageOneSQL, "OFFSET") {
		t.Fatalf("page 1 SQL = %q", pageOneSQL)
	}
	if !strings.Contains(pageTwoSQL, "LIMIT 2 OFFSET 2") {
		t.Fatalf("page 2 SQL = %q", pageTwoSQL)
	}
}

func TestAdminSharedContentScorePaginationOrdersBeforeStablePage(t *testing.T) {
	db, err := gorm.Open(mysql.New(mysql.Config{
		DSN:                       "gorm:gorm@tcp(localhost:9910)/gorm?charset=utf8&parseTime=True&loc=Local",
		SkipInitializeWithVersion: true,
	}), &gorm.Config{DryRun: true, DisableAutomaticPing: true})
	if err != nil {
		t.Fatal(err)
	}

	pageOneSQL := db.ToSQL(func(tx *gorm.DB) *gorm.DB {
		return applyAdminSharedContentScoreOrderingAndPage(tx.Table("friend_shared_content_score_records AS s"), AdminPageQuery{Page: 1, PageSize: 2}).
			Find(&[]AdminSharedContentScoreRecord{})
	})
	pageTwoSQL := db.ToSQL(func(tx *gorm.DB) *gorm.DB {
		return applyAdminSharedContentScoreOrderingAndPage(tx.Table("friend_shared_content_score_records AS s"), AdminPageQuery{Page: 2, PageSize: 2}).
			Find(&[]AdminSharedContentScoreRecord{})
	})

	const stableOrder = "ORDER BY s.score DESC, s.last_used_at DESC, s.id DESC"
	if !strings.Contains(pageOneSQL, stableOrder) || !strings.Contains(pageTwoSQL, stableOrder) {
		t.Fatalf("score pagination must use server-side stable order: page 1 = %q, page 2 = %q", pageOneSQL, pageTwoSQL)
	}
	if !strings.Contains(pageOneSQL, "LIMIT 2") || strings.Contains(pageOneSQL, "OFFSET") {
		t.Fatalf("page 1 SQL = %q", pageOneSQL)
	}
	if !strings.Contains(pageTwoSQL, "LIMIT 2 OFFSET 2") {
		t.Fatalf("page 2 SQL = %q", pageTwoSQL)
	}
}

func TestMergeAdminDailyFeatureRecordsSortsAndPaginatesNewestFirst(t *testing.T) {
	query := AdminPageQuery{Page: 1, PageSize: 2}
	firstPage, total, err := mergeAdminDailyFeatureRecords(
		query,
		[]adminDailyUIDCount{{Date: "2026-07-14", UserCount: 2}, {Date: "2026-07-16", UserCount: 4}},
		[]adminDailyUIDCount{{Date: "2026-07-16", UserCount: 3}},
		[]adminDailyUIDCount{{Date: "2026-07-15", UserCount: 5}},
		[]adminDailyUIDCount{{Date: "2026-07-15", UserCount: 2}},
		[]adminDailyUIDCount{{Date: "2026-07-16", UserCount: 1}},
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
	if firstPage[1].ExerciseSetCount != 5 || firstPage[0].UpdatedPlanCount != 1 || firstPage[1].CreatedPlanCount != 2 {
		t.Fatalf("merged action and plan counts = %#v", firstPage)
	}

	query.Page = 2
	secondPage, total, err := mergeAdminDailyFeatureRecords(
		query,
		[]adminDailyUIDCount{{Date: "2026-07-14", UserCount: 2}, {Date: "2026-07-16", UserCount: 4}},
		[]adminDailyUIDCount{{Date: "2026-07-16", UserCount: 3}},
		[]adminDailyUIDCount{{Date: "2026-07-15", UserCount: 5}},
		[]adminDailyUIDCount{{Date: "2026-07-15", UserCount: 2}},
		[]adminDailyUIDCount{{Date: "2026-07-16", UserCount: 1}},
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
	items, total, err := mergeAdminDailyFeatureRecords(AdminPageQuery{Page: 1, PageSize: 30}, nil, nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if total != 0 || items == nil || len(items) != 0 {
		t.Fatalf("items = %#v, total = %d, want non-nil empty list", items, total)
	}
}

func TestAdminDailyFeatureQueriesCountDistinctUIDsByBusinessDate(t *testing.T) {
	db, err := gorm.Open(mysql.New(mysql.Config{
		DSN:                       "gorm:gorm@tcp(localhost:9910)/gorm?charset=utf8&parseTime=True&loc=Local",
		SkipInitializeWithVersion: true,
	}), &gorm.Config{DryRun: true, DisableAutomaticPing: true})
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name       string
		table      string
		condition  string
		timeColumn string
		expected   []string
	}{
		{name: "created exercise sets", table: "exercise_set_records", timeColumn: "created_at"},
		{name: "created plans", table: "workout_data_snapshots", condition: "kind = 4", timeColumn: "created_at", expected: []string{"kind = 4"}},
		{name: "updated plans", table: "workout_data_snapshots", condition: "kind = 4 AND deleted_at IS NULL AND updated_at > created_at", timeColumn: "updated_at", expected: []string{"kind = 4 AND deleted_at IS NULL AND updated_at > created_at"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			sql := db.ToSQL(func(tx *gorm.DB) *gorm.DB {
				return adminDailyDistinctUIDQuery(tx, test.table, test.condition, test.timeColumn, AdminPageQuery{}).
					Scan(&[]adminDailyUIDCount{})
			})
			expectedParts := append([]string{
				"DATE(" + test.timeColumn + ") AS activity_date",
				"COUNT(DISTINCT uid) AS user_count",
				"GROUP BY DATE(" + test.timeColumn + ")",
			}, test.expected...)
			for _, expected := range expectedParts {
				if !strings.Contains(sql, expected) {
					t.Fatalf("daily feature SQL must contain %q: %s", expected, sql)
				}
			}
		})
	}
}

func TestAdminPlanAndWorkoutSnapshotDetailsKeepSets(t *testing.T) {
	plan := adminPlanDetailFromPB(&pb.WorkoutPlanSnapshot{
		Id: "plan-1", Title: "推举", UpdatedAt: 200,
		Exercises: []*pb.WorkoutPlanExerciseSnapshot{{
			Id: "exercise-1", ExerciseId: "bench_press", NameSnapshot: "杠铃卧推", SetCount: 3,
			Sets: []*pb.WorkoutPlanExerciseSetSnapshot{{WeightText: "60kg", RepsText: "8"}, {WeightText: "65kg", RepsText: "6"}},
		}},
	})
	if len(plan.Exercises) != 1 || plan.Exercises[0].SetCount != 3 || len(plan.Exercises[0].Sets) != 2 || plan.Exercises[0].Sets[1].WeightText != "65kg" {
		t.Fatalf("plan detail = %#v", plan)
	}

	session := adminWorkoutSessionDetailFromPB(9, &pb.WorkoutTrainingSessionSnapshot{
		SessionId: "session-1", EndedAt: 300,
		Records: []*pb.ExerciseSetRecord{
			{ExerciseId: "bench_press", ExerciseNameSnapshot: "杠铃卧推", WeightX10: 600, WeightUnit: pb.ExerciseWeightUnit_EXERCISE_WEIGHT_UNIT_KG, Reps: 8},
			{ExerciseId: "bench_press", ExerciseNameSnapshot: "杠铃卧推", WeightX10: 650, WeightUnit: pb.ExerciseWeightUnit_EXERCISE_WEIGHT_UNIT_KG, Reps: 6},
			{ExerciseId: "row", ExerciseNameSnapshot: "划船", WeightX10: 500, Reps: 10},
		},
	})
	if len(session.Actions) != 2 || session.Actions[0].SetCount != 2 || session.Actions[0].Sets[1].Reps != 6 || session.Actions[1].SetCount != 1 {
		t.Fatalf("workout detail = %#v", session)
	}
}

func TestAdminDecodeWorkoutSnapshotRejectsInvalidPayload(t *testing.T) {
	if _, ok := adminDecodeWorkoutSnapshot([]byte("not a protobuf snapshot")); ok {
		t.Fatal("invalid payload decoded successfully")
	}
	payload, err := proto.Marshal(&pb.WorkoutDataSnapshot{Kind: pb.WorkoutDataSnapshotKind_WORKOUT_DATA_SNAPSHOT_KIND_TRAINING_SESSION})
	if err != nil {
		t.Fatal(err)
	}
	decoded, ok := adminDecodeWorkoutSnapshot(payload)
	if !ok || decoded.GetKind() != pb.WorkoutDataSnapshotKind_WORKOUT_DATA_SNAPSHOT_KIND_TRAINING_SESSION {
		t.Fatalf("decoded = %#v, ok = %v", decoded, ok)
	}
}
