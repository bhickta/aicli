package video

import "math"

const courseProgressUnitLabel = "course work second"

type courseProgressPlan struct {
	transcriptUnitsByFile  map[string]int
	compressionUnitsByFile map[string]int
	missingTranscriptCount int
	totalUnits             int
}

func newCourseProgressPlan(files []string, durations map[string]float64, cacheDir string) courseProgressPlan {
	plan := courseProgressPlan{
		transcriptUnitsByFile:  make(map[string]int, len(files)),
		compressionUnitsByFile: make(map[string]int, len(files)),
	}
	for _, file := range files {
		units := durationProgressUnits(durations[file])
		plan.compressionUnitsByFile[file] = units
		plan.totalUnits += units

		cacheSRT, _, sidecarSRT := transcriptPaths(file, cacheDir)
		if fileExists(cacheSRT) || fileExists(sidecarSRT) {
			continue
		}
		plan.transcriptUnitsByFile[file] = units
		plan.missingTranscriptCount++
		plan.totalUnits += units
	}
	plan.totalUnits++
	if plan.totalUnits < 1 {
		plan.totalUnits = 1
	}
	return plan
}

func (p courseProgressPlan) transcriptUnits(file string) int {
	if units := p.transcriptUnitsByFile[file]; units > 0 {
		return units
	}
	return p.compressionUnits(file)
}

func (p courseProgressPlan) compressionUnits(file string) int {
	if units := p.compressionUnitsByFile[file]; units > 0 {
		return units
	}
	return 1
}

func (p courseProgressPlan) completedTranscriptUnits(transcribed map[string]bool) int {
	completed := 0
	for file := range transcribed {
		completed += p.transcriptUnits(file)
	}
	return completed
}

func durationProgressUnits(durationSeconds float64) int {
	if !isFinitePositive(durationSeconds) {
		return 1
	}
	return max(1, int(math.Ceil(durationSeconds)))
}

func isFinitePositive(value float64) bool {
	return value > 0 && !math.IsNaN(value) && !math.IsInf(value, 0)
}
