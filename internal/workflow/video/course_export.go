package video

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func (s *Service) exportCourseParts(ctx context.Context, targetDir string, courseDir string, outputName string, items []CourseItem, transcribed []CourseItem, skipped []string, maxMergeHours float64) (CourseResponse, error) {
	parts, err := chunkCourseItems(ctx, s, items, maxMergeHours)
	if err != nil {
		return CourseResponse{}, err
	}
	folderName := courseOutputName(targetDir, outputName)
	response := CourseResponse{CourseDir: courseDir, Compressed: items, Transcribed: transcribed, Skipped: skipped}
	multipart := len(parts) > 1
	for i, part := range parts {
		artifact := courseArtifactPaths(courseDir, folderName, multipart, i)
		if err := removeIncompleteCourseArtifact(artifact); err != nil {
			return CourseResponse{}, err
		}
		if err := s.writeCoursePart(ctx, part, artifact); err != nil {
			return CourseResponse{}, err
		}
		if i == 0 {
			response.VideoPath = artifact.videoPath
			response.SRTPath = artifact.srtPath
			response.TextPath = artifact.textPath
		}
	}
	return response, nil
}

type courseArtifact struct {
	videoPath string
	srtPath   string
	textPath  string
}

func courseArtifactPaths(courseDir string, folderName string, multipart bool, index int) courseArtifact {
	suffix := ""
	if multipart {
		suffix = fmt.Sprintf("_Part%d", index+1)
	}
	return courseArtifact{
		videoPath: filepath.Join(courseDir, folderName+suffix+"_Slideshow.mp4"),
		srtPath:   filepath.Join(courseDir, folderName+suffix+".srt"),
		textPath:  filepath.Join(courseDir, folderName+suffix+".txt"),
	}
}

func (s *Service) writeCoursePart(ctx context.Context, part []CourseItem, artifact courseArtifact) error {
	if err := s.mergeSRTs(ctx, part, artifact.srtPath); err != nil {
		return err
	}
	if fileExists(artifact.srtPath) {
		if err := s.mergeVideosWithSRT(ctx, part, artifact.srtPath, artifact.videoPath); err != nil {
			if fallbackErr := s.mergeVideosThenEmbedSRT(ctx, part, artifact); fallbackErr != nil {
				return errors.Join(err, fallbackErr)
			}
		}
	} else {
		if err := s.mergeVideos(ctx, part, artifact.videoPath); err != nil {
			return err
		}
	}
	return mergeTranscripts(part, artifact.textPath)
}

func (s *Service) mergeVideosThenEmbedSRT(ctx context.Context, part []CourseItem, artifact courseArtifact) error {
	_ = os.Remove(artifact.videoPath)
	tmpVideoPath := courseTmpVideoPath(artifact.videoPath)
	_ = os.Remove(tmpVideoPath)
	if err := s.mergeVideos(ctx, part, tmpVideoPath); err != nil {
		return err
	}
	if err := s.embedSRT(ctx, tmpVideoPath, artifact.srtPath, artifact.videoPath); err != nil {
		_ = os.Remove(tmpVideoPath)
		return err
	}
	_ = os.Remove(tmpVideoPath)
	return nil
}

func courseTmpVideoPath(videoPath string) string {
	ext := filepath.Ext(videoPath)
	return strings.TrimSuffix(videoPath, ext) + "_tmp" + ext
}

func courseOutputName(targetDir string, outputName string) string {
	name := sanitizeCourseName(outputName)
	if name == "" {
		name = sanitizeCourseName(filepath.Base(targetDir))
	}
	if name == "" {
		return "Course"
	}
	return name
}
