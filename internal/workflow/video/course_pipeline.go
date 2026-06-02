package video

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	progressmodel "github.com/bhickta/aicli/internal/progress"
	"github.com/bhickta/aicli/internal/workflow/whisper"
)

type pipelineCourseItem struct {
	index int
	item  CourseItem
}

type pipelineJob struct {
	index         int
	didTranscribe bool
}

type pipelineResult struct {
	index         int
	item          CourseItem
	didTranscribe bool
	err           error
}

type plannedCoursePart struct {
	indexes  []int
	items    []CourseItem
	artifact courseArtifact
}

func (s *Service) runCoursePipeline(
	ctx context.Context,
	targetDir string,
	courseDir string,
	cacheDir string,
	slidesDir string,
	files []string,
	durations map[string]float64,
	skipped []string,
	req CourseRequest,
	progressPlan courseProgressPlan,
	progress CourseProgressFunc,
) (CourseResponse, bool, error) {
	targetNames := courseTargetNames(files)
	items, ready, missing, err := preparePipelineCourseItems(files, targetNames, cacheDir, slidesDir)
	if err != nil {
		return CourseResponse{}, true, err
	}
	if len(missing) > 0 {
		if !whisper.CanRunFasterBatch(s.runner) || strings.TrimSpace(s.tools.WhisperCLI) == "" {
			return CourseResponse{}, false, nil
		}
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	parts := planCourseParts(targetDir, courseDir, req, items, durations)
	response := CourseResponse{CourseDir: courseDir, Skipped: skipped}
	jobs := make(chan pipelineJob, len(files))
	results := make(chan pipelineResult, len(files))
	var completedUnits atomic.Int64
	var completedTranscripts atomic.Int64
	var completedCompressed atomic.Int64
	var firstErr error
	var errMu sync.Mutex
	setErr := func(err error) {
		if err == nil {
			return
		}
		errMu.Lock()
		if firstErr == nil {
			firstErr = err
			cancel()
		}
		errMu.Unlock()
	}
	getErr := func() error {
		errMu.Lock()
		defer errMu.Unlock()
		return firstErr
	}

	enqueued := make([]bool, len(items))
	var enqueueMu sync.Mutex
	enqueue := func(index int, didTranscribe bool) {
		enqueueMu.Lock()
		if index < 0 || index >= len(enqueued) || enqueued[index] {
			enqueueMu.Unlock()
			return
		}
		enqueued[index] = true
		enqueueMu.Unlock()
		select {
		case jobs <- pipelineJob{index: index, didTranscribe: didTranscribe}:
		case <-ctx.Done():
		}
	}
	isEnqueued := func(index int) bool {
		enqueueMu.Lock()
		defer enqueueMu.Unlock()
		return index >= 0 && index < len(enqueued) && enqueued[index]
	}

	var workerWG sync.WaitGroup
	workers := normalizedCourseWorkers(courseCompressionWorkers(req), len(files))
	for range workers {
		workerWG.Add(1)
		go func() {
			defer workerWG.Done()
			for job := range jobs {
				if err := ctx.Err(); err != nil {
					return
				}
				item, err := s.compressPipelineItem(ctx, items[job.index], req)
				select {
				case results <- pipelineResult{index: job.index, item: item, didTranscribe: job.didTranscribe, err: err}:
				case <-ctx.Done():
					return
				}
				if err != nil {
					setErr(err)
					return
				}
			}
		}()
	}
	go func() {
		workerWG.Wait()
		close(results)
	}()

	aggregationDone := make(chan CourseResponse, 1)
	go func() {
		aggregationDone <- s.aggregatePipelineResults(ctx, pipelineAggregation{
			items:               items,
			parts:               parts,
			durations:           durations,
			req:                 req,
			progress:            progress,
			progressPlan:        progressPlan,
			totalUnits:          progressPlan.totalUnits,
			completedUnits:      &completedUnits,
			completedCompressed: &completedCompressed,
			results:             results,
			response:            &response,
			setErr:              setErr,
		})
	}()

	for _, readyItem := range ready {
		enqueue(readyItem.index, false)
	}

	fallback := false
	if len(missing) > 0 {
		missingFiles := make([]string, 0, len(missing))
		missingByBase := make(map[string]string, len(missing))
		indexByFile := make(map[string]int, len(missing))
		for _, item := range missing {
			missingFiles = append(missingFiles, item.item.Source)
			missingByBase[filepath.Base(item.item.Source)] = item.item.Source
			indexByFile[item.item.Source] = item.index
		}
		out, err := whisper.RunFasterBatchStreaming(ctx, s.runner, whisper.FasterBatchRequest{
			AudioPaths: missingFiles,
			OutputDir:  cacheDir,
			Model:      defaultWhisperModel(req.WhisperModel),
			Device:     req.WhisperDevice,
			Workers:    normalizedCourseWorkers(courseTranscriptWorkers(req), len(missingFiles)),
			BatchSize:  24,
			BeamSize:   1,
		}, func(line string) {
			file := transcribedLinePath(line, missingByBase)
			if file == "" {
				return
			}
			index, ok := indexByFile[file]
			if !ok {
				return
			}
			if err := ensurePipelineTranscriptCache(file, cacheDir); err != nil {
				setErr(err)
				return
			}
			currentFiles := int(completedTranscripts.Add(1))
			currentUnits := int(completedUnits.Add(int64(progressPlan.transcriptUnits(file))))
			reportCourseProgress(progress, progressmodel.Units(
				fmt.Sprintf("transcribed %d/%d video(s); queued compression: %s", currentFiles, len(missing), filepath.Base(file)),
				currentUnits,
				progressPlan.totalUnits,
				courseProgressUnitLabel,
			))
			enqueue(index, true)
		})
		if err != nil {
			if whisper.FasterBatchUnavailable(out, err) {
				fallback = true
				cancel()
			} else if !errors.Is(ctx.Err(), context.Canceled) {
				setErr(whisper.OutputError(out, err))
			}
		}
		if getErr() == nil && !fallback {
			if err := finalizePipelineMissingTranscripts(missing, cacheDir, isEnqueued, enqueue); err != nil {
				setErr(err)
			}
		}
	}

	close(jobs)
	aggregated := <-aggregationDone
	if fallback {
		return CourseResponse{}, false, nil
	}
	if err := getErr(); err != nil {
		return CourseResponse{}, true, err
	}
	reportCourseProgress(progress, progressmodel.Units("completed course video, subtitles, and transcript", progressPlan.totalUnits, progressPlan.totalUnits, courseProgressUnitLabel))
	return aggregated, true, nil
}

func finalizePipelineMissingTranscripts(missing []pipelineCourseItem, cacheDir string, isEnqueued func(int) bool, enqueue func(int, bool)) error {
	for _, item := range missing {
		if isEnqueued(item.index) {
			continue
		}
		if err := ensurePipelineTranscriptCache(item.item.Source, cacheDir); err != nil {
			return err
		}
		enqueue(item.index, true)
	}
	return nil
}

func ensurePipelineTranscriptCache(file string, cacheDir string) error {
	cacheSRT, cacheText, _ := transcriptPaths(file, cacheDir)
	if !fileExists(cacheText) && fileExists(cacheSRT) {
		if err := writeTranscriptText(cacheSRT, cacheText); err != nil {
			return err
		}
	}
	if !fileExists(cacheSRT) || !fileExists(cacheText) {
		return errors.New("faster-whisper did not produce both .srt and .txt for " + filepath.Base(file))
	}
	return nil
}

type pipelineAggregation struct {
	items               []CourseItem
	parts               []plannedCoursePart
	durations           map[string]float64
	req                 CourseRequest
	progress            CourseProgressFunc
	progressPlan        courseProgressPlan
	totalUnits          int
	completedUnits      *atomic.Int64
	completedCompressed *atomic.Int64
	results             <-chan pipelineResult
	response            *CourseResponse
	setErr              func(error)
}

func (s *Service) aggregatePipelineResults(ctx context.Context, state pipelineAggregation) CourseResponse {
	ready := make([]bool, len(state.items))
	exported := make([]bool, len(state.parts))
	for res := range state.results {
		if res.err != nil {
			state.setErr(res.err)
			continue
		}
		state.items[res.index] = res.item
		ready[res.index] = true
		if res.didTranscribe {
			state.response.Transcribed = append(state.response.Transcribed, CourseItem{
				Source:     res.item.Source,
				SRTPath:    res.item.SRTPath,
				TextPath:   res.item.TextPath,
				TargetName: res.item.TargetName,
			})
		}
		compressed := int(state.completedCompressed.Add(1))
		currentUnits := int(state.completedUnits.Add(int64(state.progressPlan.compressionUnits(res.item.Source))))
		reportCourseProgress(state.progress, progressmodel.Units(
			fmt.Sprintf("compressed %d/%d video(s); exporting ready parts", compressed, len(state.items)),
			currentUnits,
			state.totalUnits,
			courseProgressUnitLabel,
		))
		s.exportReadyCourseParts(ctx, state, ready, exported)
	}
	s.exportReadyCourseParts(ctx, state, ready, exported)
	state.response.Compressed = append([]CourseItem(nil), state.items...)
	return *state.response
}

func (s *Service) exportReadyCourseParts(ctx context.Context, state pipelineAggregation, ready []bool, exported []bool) {
	for partIndex := range state.parts {
		if exported[partIndex] {
			continue
		}
		part := state.parts[partIndex]
		if !coursePartReady(part, ready) {
			break
		}
		reportCourseProgress(state.progress, progressmodel.Units(
			fmt.Sprintf("merging verified course part %d/%d", partIndex+1, len(state.parts)),
			activeCourseProgressUnits(int(state.completedUnits.Load())+1, state.totalUnits),
			state.totalUnits,
			courseProgressUnitLabel,
		))
		if err := removeIncompleteCourseArtifact(part.artifact); err != nil {
			state.setErr(err)
			return
		}
		if err := s.writeCoursePart(ctx, part.items, part.artifact); err != nil {
			state.setErr(err)
			return
		}
		if err := s.verifyCoursePart(ctx, part.items, part.artifact, state.durations); err != nil {
			state.setErr(err)
			return
		}
		if partIndex == 0 {
			state.response.VideoPath = part.artifact.videoPath
			state.response.SRTPath = part.artifact.srtPath
			state.response.TextPath = part.artifact.textPath
		}
		if state.req.CleanupVerified {
			cleaned, err := cleanupVerifiedCoursePart(part.items)
			if err != nil {
				state.setErr(err)
				return
			}
			state.response.Cleaned = append(state.response.Cleaned, cleaned...)
		}
		exported[partIndex] = true
		reportCourseProgress(state.progress, progressmodel.Units(
			fmt.Sprintf("verified course part %d/%d", partIndex+1, len(state.parts)),
			activeCourseProgressUnits(int(state.completedUnits.Load())+1, state.totalUnits),
			state.totalUnits,
			courseProgressUnitLabel,
		))
	}
}

func activeCourseProgressUnits(completed int, total int) int {
	if total <= 0 {
		return 0
	}
	if completed < 0 {
		completed = 0
	}
	capUnits := total - 1
	if total >= 100 {
		capUnits = min(capUnits, int(math.Floor(float64(total)*0.99)))
	}
	if capUnits < 0 {
		capUnits = 0
	}
	return min(completed, capUnits)
}

func preparePipelineCourseItems(files []string, targetNames []string, cacheDir string, slidesDir string) ([]CourseItem, []pipelineCourseItem, []pipelineCourseItem, error) {
	if err := os.MkdirAll(slidesDir, 0o755); err != nil {
		return nil, nil, nil, err
	}
	items := make([]CourseItem, len(files))
	ready := []pipelineCourseItem{}
	missing := []pipelineCourseItem{}
	for i, file := range files {
		cacheSRT, cacheText, sidecarSRT := transcriptPaths(file, cacheDir)
		if !fileExists(cacheSRT) && fileExists(sidecarSRT) {
			if err := copyFile(sidecarSRT, cacheSRT); err != nil {
				return nil, nil, nil, err
			}
		}
		if fileExists(cacheSRT) && !fileExists(cacheText) {
			if err := writeTranscriptText(cacheSRT, cacheText); err != nil {
				return nil, nil, nil, err
			}
		}
		item := CourseItem{
			Source:     file,
			Output:     filepath.Join(slidesDir, targetNames[i]+"_slideshow.mp4"),
			SRTPath:    cacheSRT,
			TextPath:   cacheText,
			TargetName: targetNames[i],
		}
		items[i] = item
		pipelineItem := pipelineCourseItem{index: i, item: item}
		if fileExists(cacheSRT) && fileExists(cacheText) {
			ready = append(ready, pipelineItem)
			continue
		}
		missing = append(missing, pipelineItem)
	}
	return items, ready, missing, nil
}

func (s *Service) compressPipelineItem(ctx context.Context, item CourseItem, req CourseRequest) (CourseItem, error) {
	if _, statErr := os.Stat(item.Output); statErr == nil {
		return item, nil
	}
	compressReq := courseCompressRequest(item.Source, item.Output, item.SRTPath, item.TargetName, req)
	output, out, err := s.compress(ctx, compressReq)
	if err != nil {
		return CourseItem{}, errors.New(strings.TrimSpace(string(out)) + ": " + err.Error())
	}
	item.Output = output
	return item, nil
}

func planCourseParts(targetDir string, courseDir string, req CourseRequest, items []CourseItem, durations map[string]float64) []plannedCoursePart {
	folderName := courseOutputName(targetDir, req.OutputName)
	existing := []existingCoursePart{}
	if req.CleanupVerified {
		existing = existingVerifiedCourseParts(courseDir, folderName)
	}
	groups := coursePartGroupsForResume(items, durations, req.MaxMergeHours, existing)
	occupied := existingCoursePartNumbers(existing)
	allocator := newCoursePartAllocator(occupied)
	offset := len(occupied)
	multipart := len(groups) > 1 || offset > 0 || (req.CleanupVerified && req.MaxMergeHours > 0)
	parts := make([]plannedCoursePart, 0, len(groups))
	for _, group := range groups {
		indexes := make([]int, len(group))
		partItems := make([]CourseItem, len(group))
		for j, item := range group {
			indexes[j] = item.index
			partItems[j] = item.item
		}
		parts = append(parts, plannedCoursePart{
			indexes:  indexes,
			items:    partItems,
			artifact: courseArtifactPaths(courseDir, folderName, multipart, allocator.next()),
		})
	}
	return parts
}

func chunkCourseItemsByDuration(items []CourseItem, durations map[string]float64, maxMergeHours float64) [][]pipelineCourseItem {
	pipelineItems := make([]pipelineCourseItem, len(items))
	for i, item := range items {
		pipelineItems[i] = pipelineCourseItem{index: i, item: item}
	}
	return chunkPipelineCourseItemsByDuration(pipelineItems, durations, maxMergeHours)
}

func chunkPipelineCourseItemsByDuration(items []pipelineCourseItem, durations map[string]float64, maxMergeHours float64) [][]pipelineCourseItem {
	groups := [][]pipelineCourseItem{}
	if len(items) == 0 {
		return groups
	}
	if maxMergeHours <= 0 {
		return [][]pipelineCourseItem{append([]pipelineCourseItem(nil), items...)}
	}
	limit := maxMergeHours * 3600
	current := []pipelineCourseItem{}
	currentSeconds := 0.0
	for _, item := range items {
		duration := durations[item.item.Source]
		if len(current) > 0 && currentSeconds+duration > limit {
			groups = append(groups, current)
			current = nil
			currentSeconds = 0
		}
		current = append(current, item)
		currentSeconds += duration
	}
	if len(current) > 0 {
		groups = append(groups, current)
	}
	return groups
}

func coursePartReady(part plannedCoursePart, ready []bool) bool {
	for _, index := range part.indexes {
		if index < 0 || index >= len(ready) || !ready[index] {
			return false
		}
	}
	return true
}

type existingCoursePart struct {
	number   int
	segments []string
}

type coursePartAllocator struct {
	occupied map[int]bool
	nextPart int
}

func newCoursePartAllocator(occupied map[int]bool) coursePartAllocator {
	return coursePartAllocator{occupied: occupied, nextPart: 1}
}

func (a *coursePartAllocator) next() int {
	for a.occupied[a.nextPart] {
		a.nextPart++
	}
	part := a.nextPart
	a.occupied[part] = true
	a.nextPart++
	return part - 1
}

func existingVerifiedCourseParts(courseDir string, folderName string) []existingCoursePart {
	pattern := filepath.Join(courseDir, folderName+"_Part*_Slideshow.mp4")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil
	}
	re := regexp.MustCompile(`_Part([0-9]+)_Slideshow\.mp4$`)
	parts := []existingCoursePart{}
	for _, match := range matches {
		matches := re.FindStringSubmatch(filepath.Base(match))
		if len(matches) != 2 {
			continue
		}
		number, err := strconv.Atoi(matches[1])
		if err != nil || number <= 0 {
			continue
		}
		artifact := courseArtifactPaths(courseDir, folderName, true, number-1)
		if !courseArtifactComplete(artifact) {
			continue
		}
		parts = append(parts, existingCoursePart{
			number:   number,
			segments: coursePartSegments(artifact.textPath),
		})
	}
	sort.Slice(parts, func(i, j int) bool {
		return parts[i].number < parts[j].number
	})
	return parts
}

func existingCoursePartNumbers(parts []existingCoursePart) map[int]bool {
	occupied := make(map[int]bool, len(parts))
	for _, part := range parts {
		occupied[part.number] = true
	}
	return occupied
}

func coursePartGroupsForResume(
	items []CourseItem,
	durations map[string]float64,
	maxMergeHours float64,
	existing []existingCoursePart,
) [][]pipelineCourseItem {
	if len(existing) == 0 {
		return chunkCourseItemsByDuration(items, durations, maxMergeHours)
	}

	occupiedSegments := map[string]bool{}
	for _, part := range existing {
		for _, segment := range part.segments {
			occupiedSegments[segment] = true
		}
	}

	pending := make([]pipelineCourseItem, 0, len(items))
	for i, item := range items {
		if occupiedSegments[item.TargetName] {
			continue
		}
		pending = append(pending, pipelineCourseItem{index: i, item: item})
	}

	groups := [][]pipelineCourseItem{}
	cursor := 0
	for _, part := range existing {
		if len(part.segments) == 0 {
			continue
		}
		firstSegment := part.segments[0]
		gap := []pipelineCourseItem{}
		for cursor < len(pending) && pending[cursor].item.TargetName < firstSegment {
			gap = append(gap, pending[cursor])
			cursor++
		}
		if len(gap) > 0 {
			groups = append(groups, chunkPipelineCourseItemsByDuration(gap, durations, maxMergeHours)...)
		}
		for cursor < len(pending) && occupiedSegments[pending[cursor].item.TargetName] {
			cursor++
		}
	}
	if cursor < len(pending) {
		groups = append(groups, chunkPipelineCourseItemsByDuration(pending[cursor:], durations, maxMergeHours)...)
	}
	return groups
}

func coursePartSegments(textPath string) []string {
	data, err := os.ReadFile(textPath)
	if err != nil {
		return nil
	}
	re := regexp.MustCompile(`(?m)^--- Segment: (.+) ---$`)
	matches := re.FindAllStringSubmatch(string(data), -1)
	segments := make([]string, 0, len(matches))
	for _, match := range matches {
		if len(match) == 2 {
			segments = append(segments, match[1])
		}
	}
	return segments
}

func courseArtifactComplete(artifact courseArtifact) bool {
	for _, path := range []string{artifact.videoPath, artifact.srtPath, artifact.textPath} {
		info, err := os.Stat(path)
		if err != nil || info.Size() == 0 {
			return false
		}
	}
	return true
}

func removeIncompleteCourseArtifact(artifact courseArtifact) error {
	if courseArtifactComplete(artifact) {
		return nil
	}
	for _, path := range []string{artifact.videoPath, artifact.srtPath, artifact.textPath, courseTmpVideoPath(artifact.videoPath)} {
		if path == "" {
			continue
		}
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func (s *Service) verifyCoursePart(ctx context.Context, part []CourseItem, artifact courseArtifact, durations map[string]float64) error {
	for _, path := range []string{artifact.videoPath, artifact.srtPath, artifact.textPath} {
		info, err := os.Stat(path)
		if err != nil {
			return err
		}
		if info.Size() == 0 {
			return fmt.Errorf("course artifact is empty: %s", path)
		}
	}
	actualDuration, err := s.duration(ctx, artifact.videoPath)
	if err != nil {
		return err
	}
	expectedDuration := 0.0
	for _, item := range part {
		expectedDuration += durations[item.Source]
	}
	if expectedDuration > 0 {
		tolerance := math.Max(120, expectedDuration*0.005)
		if math.Abs(actualDuration-expectedDuration) > tolerance {
			return fmt.Errorf("verified video duration mismatch for %s: got %.2fs, want %.2fs +/- %.2fs", artifact.videoPath, actualDuration, expectedDuration, tolerance)
		}
	}
	return nil
}

func cleanupVerifiedCoursePart(part []CourseItem) ([]string, error) {
	seen := map[string]bool{}
	cleaned := []string{}
	for _, item := range part {
		paths := []string{item.Source, item.Output, item.SRTPath, item.TextPath}
		if _, _, sidecarSRT := transcriptPaths(item.Source, filepath.Dir(item.SRTPath)); sidecarSRT != "" {
			paths = append(paths, sidecarSRT)
		}
		for _, path := range paths {
			if path == "" || seen[path] {
				continue
			}
			seen[path] = true
			removed, err := removeCoursePath(path)
			if err != nil {
				return cleaned, err
			}
			if removed {
				cleaned = append(cleaned, path)
			}
		}
	}
	for _, item := range part {
		removeEmptyCourseDir(filepath.Dir(item.Output))
		removeEmptyCourseDir(filepath.Dir(item.SRTPath))
	}
	return cleaned, nil
}

func removeCoursePath(path string) (bool, error) {
	if strings.TrimSpace(path) == "" {
		return false, nil
	}
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func removeEmptyCourseDir(path string) {
	if strings.TrimSpace(path) == "" || path == "." || path == string(filepath.Separator) {
		return
	}
	_ = os.Remove(path)
}

func defaultWhisperModel(model string) string {
	if strings.TrimSpace(model) == "" {
		return "large-v3"
	}
	return model
}
