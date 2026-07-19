package mysqlmodel

import "testing"

func TestCustomExerciseToPBPreservesIntroduction(t *testing.T) {
	remote := CustomExerciseToPB(&CustomExercise{
		UID:            7,
		LocalID:        "custom-1",
		Name:           "自定义推举",
		CategoryKey:    "exercise_category_shoulders",
		SubcategoryKey: "exercise_subcategory_front_deltoid",
		TypeKey:        "exercise_type_dumbbell",
		Introduction:   "保持核心稳定",
	})
	if remote.GetIntroduction() != "保持核心稳定" {
		t.Fatalf("introduction = %q", remote.GetIntroduction())
	}
}
