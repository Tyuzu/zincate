package feed

import (
	"fmt"
	"io"
	"math"
	"naevis/mq"
	"naevis/utils"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/disintegration/imaging"
)

// Directory to store uploaded images/videos
const feedVideoUploadDir = "./static/postpic/"

// Generic function to ensure directory existence
func ensureDir(dir string) error {
	return os.MkdirAll(dir, 0755)
}

// Generic function to save an uploaded file
func saveUploadedFile(src io.Reader, destPath string) error {
	out, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, src)
	return err
}

// Common function to generate a unique file path
func generateFilePath(baseDir, uniqueID, extension string) string {
	return filepath.Join(baseDir, fmt.Sprintf("%s.%s", uniqueID, extension))
}

// Handles video uploads and processing
func saveUploadedVideoFile(r *http.Request, formKey string) ([]int, []string, []string, error) {
	files := r.MultipartForm.File[formKey]
	if len(files) == 0 {
		return nil, nil, nil, nil // No file to process
	}

	file := files[0]
	src, err := file.Open()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to open video file: %w", err)
	}
	defer src.Close()

	uniqueID := utils.GenerateID(16)
	uploadDir := filepath.Join(feedVideoUploadDir, uniqueID)
	originalFilePath := generateFilePath(uploadDir, uniqueID, "mp4")

	// Ensure directory exists
	if err := ensureDir(uploadDir); err != nil {
		return nil, nil, nil, fmt.Errorf("failed to create upload directory: %w", err)
	}

	// Save the original file
	if err := saveUploadedFile(src, originalFilePath); err != nil {
		return nil, nil, nil, fmt.Errorf("failed to save video file: %w", err)
	}

	// Ensure cleanup on failure
	defer func() {
		if err != nil {
			os.RemoveAll(uploadDir)
		}
	}()

	origWidth, origHeight, err := getVideoDimensions(originalFilePath)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to get video dimensions: %w", err)
	}

	// Define available resolutions
	resolutions := []struct {
		Label  string
		Width  int
		Height int
	}{
		{"4320p", 7680, 4320}, {"2160p", 3840, 2160}, {"1440p", 2560, 1440},
		{"1080p", 1920, 1080}, {"720p", 1280, 720}, {"480p", 854, 480},
		{"360p", 640, 360}, {"240p", 426, 240}, {"144p", 256, 144},
	}

	var highestResolutionPath string
	var availableResolutions []int

	for _, res := range resolutions {
		newWidth, newHeight := fitResolution(origWidth, origHeight, res.Width, res.Height)
		if newWidth > origWidth || newHeight > origHeight {
			continue
		}

		outputFilePath := generateFilePath(uploadDir, uniqueID+"-"+res.Label, "mp4")
		outputPosterPath := generateFilePath(uploadDir, uniqueID+"-"+res.Label, "jpg")

		if err := processVideoResolution(originalFilePath, outputFilePath, fmt.Sprintf("%dx%d", newWidth, newHeight)); err != nil {
			fmt.Printf("Skipping %s due to error: %v\n", res.Label, err)
			continue
		}

		if err := CreatePoster(outputFilePath, outputPosterPath, "00:00:01"); err != nil {
			fmt.Printf("Skipping %s poster due to error: %v\n", res.Label, err)
			continue
		}

		highestResolutionPath = "/postpic/" + uniqueID + "/" + filepath.Base(outputFilePath)
		availableResolutions = append(availableResolutions, newHeight)
	}

	defaultPosterPath := generateFilePath(uploadDir, uniqueID, "jpg")
	if err := CreatePoster(originalFilePath, defaultPosterPath, "00:00:01"); err != nil {
		return nil, nil, nil, fmt.Errorf("failed to create default video poster: %w", err)
	}

	// Generate subtitles asynchronously
	go createSubtitleFile(uniqueID)

	// Notify MQ system
	m := mq.Index{}
	mq.Notify("postpics-uploaded", m)

	return availableResolutions, []string{highestResolutionPath}, []string{uniqueID}, nil
}

// Handles image uploads and processing
func saveUploadedFiles(r *http.Request, formKey, fileType string) ([]string, []string, error) {
	files := r.MultipartForm.File[formKey]
	if len(files) == 0 {
		return nil, nil, nil // No files to process
	}

	var savedPaths, savedNames []string

	for _, file := range files {
		src, err := file.Open()
		if err != nil {
			return nil, nil, fmt.Errorf("failed to open %s file: %w", fileType, err)
		}
		defer src.Close()

		img, err := imaging.Decode(src)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to decode image: %w", err)
		}

		uniqueID := utils.GenerateID(16)
		fileName := uniqueID + ".jpg"
		originalPath := generateFilePath(feedVideoUploadDir, uniqueID, "jpg")
		thumbnailPath := generateFilePath(feedVideoUploadDir+"/thumb", uniqueID, "jpg")

		// Ensure upload directories exist
		if err := ensureDir(filepath.Dir(originalPath)); err != nil {
			return nil, nil, fmt.Errorf("failed to create upload directory: %w", err)
		}
		if err := ensureDir(filepath.Dir(thumbnailPath)); err != nil {
			return nil, nil, fmt.Errorf("failed to create thumbnail directory: %w", err)
		}

		// Save original image
		if err := imaging.Save(img, originalPath); err != nil {
			return nil, nil, fmt.Errorf("failed to save original image: %w", err)
		}

		// Create and save thumbnail
		// thumbImg := imaging.Resize(img, 720, 0, imaging.Lanczos)
		thumbImg := imaging.Resize(img, 300, 0, imaging.Lanczos)
		if err := imaging.Save(thumbImg, thumbnailPath); err != nil {
			return nil, nil, fmt.Errorf("failed to save thumbnail: %w", err)
		}

		// utils.CreateThumb(uniqueID, feedVideoUploadDir, ".jpg", 300, 300)

		// Store only the thumbnail path in savedPaths
		savedPaths = append(savedPaths, "/postpic/"+fileName)
		savedNames = append(savedNames, uniqueID)
	}

	// Notify MQ system
	m := mq.Index{}
	mq.Notify("postpics-uploaded", m)
	mq.Notify("thumbnail-created", m)

	return savedPaths, savedNames, nil
}

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

func createSubtitleFile(uniqueID string) {
	// Example subtitles
	subtitles := []Subtitle{
		{
			Index:   1,
			Start:   "00:00:00.000",
			End:     "00:00:01.000",
			Content: "Welcome to the video!",
		},
		{
			Index:   2,
			Start:   "00:00:01.001",
			End:     "00:00:02.000",
			Content: "In this video, we'll learn how to create subtitles in Go.",
		},
		{
			Index:   3,
			Start:   "00:00:02.001",
			End:     "00:00:03.000",
			Content: "Let's get started!",
		},
	}

	var lang = "english"

	// File name for the .vtt file
	// fileName := "example.vtt"
	fileName := fmt.Sprintf("./static/postpic/%s/%s-%s.vtt", uniqueID, uniqueID, lang)

	// Create the VTT file
	err := createVTTFile(fileName, subtitles)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Subtitle file %s created successfully!\n", fileName)
}

// Subtitle represents a single subtitle entry
type Subtitle struct {
	Index   int
	Start   string // Start time in format "hh:mm:ss.mmm"
	End     string // End time in format "hh:mm:ss.mmm"
	Content string // Subtitle text
}

func createVTTFile(fileName string, subtitles []Subtitle) error {
	// Create or overwrite the file
	file, err := os.Create(fileName)
	if err != nil {
		return fmt.Errorf("failed to create file: %v", err)
	}
	defer file.Close()

	// Write the WebVTT header
	_, err = file.WriteString("WEBVTT\n\n")
	if err != nil {
		return fmt.Errorf("failed to write header: %v", err)
	}

	// Write each subtitle entry
	for _, subtitle := range subtitles {
		entry := fmt.Sprintf("%d\n%s --> %s\n%s\n\n",
			subtitle.Index,
			subtitle.Start,
			subtitle.End,
			subtitle.Content,
		)
		_, err := file.WriteString(entry)
		if err != nil {
			return fmt.Errorf("failed to write subtitle entry: %v", err)
		}
	}

	return nil
}
