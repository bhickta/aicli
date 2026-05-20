package progress

import "time"

const (
	ModeDeterminate   = "determinate"
	ModeIndeterminate = "indeterminate"
	ModeTimed         = "timed"
)

type Update struct {
	Stage          string
	Mode           string
	CompletedUnits int
	TotalUnits     int
	UnitLabel      string
	StartedAt      time.Time
	EndsAt         time.Time
}

type Func func(Update)

func Indeterminate(stage string) Update {
	return Update{Stage: stage, Mode: ModeIndeterminate}
}

func Units(stage string, completed int, total int, label string) Update {
	return Update{
		Stage:          stage,
		Mode:           ModeDeterminate,
		CompletedUnits: completed,
		TotalUnits:     total,
		UnitLabel:      label,
	}
}

func Timed(stage string, startedAt time.Time, endsAt time.Time) Update {
	return Update{
		Stage:     stage,
		Mode:      ModeTimed,
		StartedAt: startedAt,
		EndsAt:    endsAt,
	}
}

func Step(stage string, currentStep int, totalSteps int) Update {
	return Units(stage, currentStep, totalSteps, "step")
}
