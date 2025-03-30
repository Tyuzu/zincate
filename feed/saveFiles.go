package feed

import (
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"

	"naevis/mq"
	"naevis/utils"

	"github.com/disintegration/imaging"
)

// Directory to store uploaded images/videos
const feedVideoUploadDir = "./static/postpic/"

// -------------------------
// Generic helper functions
// -------------------------

// ensureDir makes sure the provided directory exists.
func ensureDir(dir string) error {
	return os.MkdirAll(dir, 0755)
}

// saveUploadedFile writes from the given io.Reader into destPath.
func saveUploadedFile(src io.Reader, destPath string) error {
	out, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, src)
	return err
}

// generateFilePath builds a file path based on a baseDir, a uniqueID, and a file extension.
func generateFilePath(baseDir, uniqueID, extension string) string {
	return filepath.Join(baseDir, fmt.Sprintf("%s.%s", uniqueID, extension))
}

// -------------------------------
// Video Upload Processing Helpers
// -------------------------------

// getFirstFile returns the first uploaded file for the given form key.
func getFirstFile(r *http.Request, formKey string) (*multipart.FileHeader, error) {
	files := r.MultipartForm.File[formKey]
	if len(files) == 0 {
		return nil, nil // No file uploaded
	}
	return files[0], nil
}

// saveVideoFile saves the uploaded video file to disk.
func saveVideoFile(file *multipart.FileHeader, uploadDir, uniqueID, postID, userID string) (string, error) {
	originalFilePath := generateFilePath(uploadDir, uniqueID, "mp4")
	src, err := file.Open()
	if err != nil {
		return "", fmt.Errorf("failed to open video file: %w", err)
	}
	defer src.Close()

	if err := saveUploadedFile(src, originalFilePath); err != nil {
		return "", fmt.Errorf("failed to save video file: %w", err)
	}

	UploadFile(src, originalFilePath, userID, postID)

	return originalFilePath, nil
}

// processVideoResolutions creates versions of the video at multiple resolutions.
// It returns a slice of available resolution heights and the path of the highest resolution file.
func processVideoResolutions(originalFilePath, uploadDir, uniqueID string, origWidth, origHeight int) ([]int, string) {
	// Define available resolutions.
	resolutions := []struct {
		Label  string
		Width  int
		Height int
	}{
		{"4320p", 7680, 4320}, {"2160p", 3840, 2160}, {"1440p", 2560, 1440},
		{"1080p", 1920, 1080}, {"720p", 1280, 720}, {"480p", 854, 480},
		{"360p", 640, 360}, {"240p", 426, 240}, {"144p", 256, 144},
	}

	var availableResolutions []int
	var highestResolutionPath string

	for _, res := range resolutions {
		newWidth, newHeight := fitResolution(origWidth, origHeight, res.Width, res.Height)
		// Only downscale (avoid upscaling)
		if newWidth > origWidth || newHeight > origHeight {
			continue
		}

		outputFilePath := generateFilePath(uploadDir, uniqueID+"-"+res.Label, "mp4")
		outputPosterPath := generateFilePath(uploadDir, uniqueID+"-"+res.Label, "jpg")

		// processVideoResolution is assumed to perform the resolution conversion.
		if err := processVideoResolution(originalFilePath, outputFilePath, fmt.Sprintf("%dx%d", newWidth, newHeight)); err != nil {
			fmt.Printf("Skipping %s due to error: %v\n", res.Label, err)
			continue
		}

		// Create a poster image for this resolution.
		if err := CreatePoster(outputFilePath, outputPosterPath, "00:00:01"); err != nil {
			fmt.Printf("Skipping %s poster due to error: %v\n", res.Label, err)
			continue
		}

		highestResolutionPath = "/postpic/" + uniqueID + "/" + filepath.Base(outputFilePath)
		availableResolutions = append(availableResolutions, newHeight)
	}

	return availableResolutions, highestResolutionPath
}

// createDefaultPoster creates a default poster image from the original video.
func createDefaultPoster(originalFilePath, uploadDir, uniqueID string) error {
	defaultPosterPath := generateFilePath(uploadDir, uniqueID, "jpg")
	return CreatePoster(originalFilePath, defaultPosterPath, "00:00:01")
}

// ------------------------
// Main Video Upload Handler
// ------------------------

// saveUploadedVideoFile handles video file uploads and processing.
func saveUploadedVideoFile(r *http.Request, formKey, postID, userID string) ([]int, []string, []string, error) {
	// Retrieve the first file for the given form key.
	file, err := getFirstFile(r, formKey)
	if err != nil {
		return nil, nil, nil, err
	}
	if file == nil {
		return nil, nil, nil, nil // No file uploaded
	}

	uniqueID := utils.GenerateID(16)
	uploadDir := filepath.Join(feedVideoUploadDir, uniqueID)

	// Ensure the upload directory exists.
	if err := ensureDir(uploadDir); err != nil {
		return nil, nil, nil, fmt.Errorf("failed to create upload directory: %w", err)
	}

	// Save the original video file.
	originalFilePath, err := saveVideoFile(file, uploadDir, uniqueID, postID, userID)
	if err != nil {
		os.RemoveAll(uploadDir) // Cleanup on failure.
		return nil, nil, nil, err
	}

	// Get the original video dimensions.
	origWidth, origHeight, err := getVideoDimensions(originalFilePath)
	if err != nil {
		os.RemoveAll(uploadDir)
		return nil, nil, nil, fmt.Errorf("failed to get video dimensions: %w", err)
	}

	// Process various resolutions.
	availableResolutions, highestResPath := processVideoResolutions(originalFilePath, uploadDir, uniqueID, origWidth, origHeight)

	// Create a default poster image.
	if err := createDefaultPoster(originalFilePath, uploadDir, uniqueID); err != nil {
		os.RemoveAll(uploadDir)
		return nil, nil, nil, fmt.Errorf("failed to create default video poster: %w", err)
	}

	// Generate subtitles asynchronously.
	go createSubtitleFile(uniqueID)

	// Notify MQ system.
	m := mq.Index{}
	mq.Notify("postpics-uploaded", m)

	return availableResolutions, []string{highestResPath}, []string{uniqueID}, nil
}

// ------------------------
// Image Upload Processing Helpers
// ------------------------

// processSingleImageUpload handles processing for one image file.
// It returns the thumbnail URL, the uniqueID of the image, or an error.
func processSingleImageUpload(file *multipart.FileHeader, postID, userID string) (string, string, error) {
	src, err := file.Open()
	if err != nil {
		return "", "", fmt.Errorf("failed to open image file: %w", err)
	}
	defer src.Close()

	img, err := imaging.Decode(src)
	if err != nil {
		return "", "", fmt.Errorf("failed to decode image: %w", err)
	}

	uniqueID := utils.GenerateID(16)
	fileName := uniqueID + ".jpg"

	// Define paths for the original image and its thumbnail.
	originalPath := generateFilePath(feedVideoUploadDir, uniqueID, "jpg")
	thumbDir := filepath.Join(feedVideoUploadDir, "thumb")
	thumbnailPath := generateFilePath(thumbDir, uniqueID, "jpg")

	// Ensure directories exist.
	if err := ensureDir(filepath.Dir(originalPath)); err != nil {
		return "", "", fmt.Errorf("failed to create upload directory: %w", err)
	}
	if err := ensureDir(thumbDir); err != nil {
		return "", "", fmt.Errorf("failed to create thumbnail directory: %w", err)
	}

	// Save the original image.
	if err := imaging.Save(img, originalPath); err != nil {
		return "", "", fmt.Errorf("failed to save original image: %w", err)
	}

	// Create and save thumbnail.
	thumbImg := imaging.Resize(img, 300, 0, imaging.Lanczos)
	if err := imaging.Save(thumbImg, thumbnailPath); err != nil {
		return "", "", fmt.Errorf("failed to save thumbnail: %w", err)
	}

	UploadFile(src, "/postpic/"+fileName, userID, postID)

	// Return the URL path for the thumbnail and the unique ID.
	return "/postpic/" + fileName, uniqueID, nil
}

// ------------------------
// Main Image Upload Handler
// ------------------------

// saveUploadedFiles handles image uploads and processing.
func saveUploadedFiles(r *http.Request, formKey, fileType, postID, userID string) ([]string, []string, error) {
	files := r.MultipartForm.File[formKey]
	if len(files) == 0 {
		return nil, nil, nil // No files to process.
	}

	var savedPaths, savedNames []string

	// Process each file.
	for _, file := range files {
		thumbPath, uniqueID, err := processSingleImageUpload(file, postID, userID)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to process %s file: %w", fileType, err)
		}
		savedPaths = append(savedPaths, thumbPath)
		savedNames = append(savedNames, uniqueID)
	}

	// Notify MQ system.
	m := mq.Index{}
	mq.Notify("postpics-uploaded", m)
	mq.Notify("thumbnail-created", m)

	return savedPaths, savedNames, nil
}
