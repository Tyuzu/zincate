package feed

import (
	"fmt"
	"math"
	"os/exec"
	"strings"
)

// Adjusts resolution while maintaining aspect ratio
func fitResolution(origW, origH, maxW, maxH int) (int, int) {
	// Scale down to fit within maxW and maxH while keeping aspect ratio
	ratio := math.Min(float64(maxW)/float64(origW), float64(maxH)/float64(origH))
	newW := int(float64(origW) * ratio)
	newH := int(float64(origH) * ratio)
	return newW, newH
}

func getVideoDimensions(videoPath string) (int, int, error) {
	cmd := exec.Command("ffprobe",
		"-v", "error",
		"-select_streams", "v:0",
		"-show_entries", "stream=width,height",
		"-of", "csv=p=0",
		videoPath,
	)

	output, err := cmd.Output()
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get video dimensions: %w", err)
	}

	data := strings.TrimSpace(string(output))
	var width, height int
	_, err = fmt.Sscanf(data, "%d,%d", &width, &height)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to parse video dimensions: %w", err)
	}

	return width, height, nil
}

// Processes video into a specific resolution using FFMPEG
func processVideoResolution(inputPath, outputPath, size string) error {
	cmd := exec.Command(
		"ffmpeg", "-i", inputPath,
		"-vf", fmt.Sprintf("scale=%s", size),
		"-c:v", "libx264", "-crf", "23",
		"-preset", "veryfast",
		"-c:a", "aac", "-b:a", "128k",
		"-movflags", "+faststart",
		outputPath,
	)
	return cmd.Run()
}

// Creates a poster (thumbnail) from a video at a given time
func CreatePoster(videoPath, posterPath, timestamp string) error {
	cmd := exec.Command(
		"ffmpeg", "-i", videoPath,
		"-ss", timestamp, "-vframes", "1",
		"-q:v", "2", posterPath,
	)
	return cmd.Run()
}
