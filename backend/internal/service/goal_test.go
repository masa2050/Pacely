package service

import (
	"errors"
	"testing"
	"time"

	"github.com/masa2050/pacely/backend/internal/model"
)

func TestGoalInputValidateRejectsUnknownGoalType(t *testing.T) {
	date := model.NewDate(time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC))
	for _, goalType := range []string{"", "foo"} {
		in := GoalInput{GoalType: goalType, TargetTimeSec: 1000, TargetDate: date}
		if err := in.validate(); !errors.Is(err, ErrInvalidInput) {
			t.Errorf("goal_type=%q: ErrInvalidInput を期待したが %v", goalType, err)
		}
	}
	ok := GoalInput{GoalType: "5km", TargetTimeSec: 1000, TargetDate: date}
	if err := ok.validate(); err != nil {
		t.Errorf("既知のgoal_typeが弾かれた: %v", err)
	}
}
