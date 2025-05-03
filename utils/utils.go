package utils

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"log"
	"math"
	rndm "math/rand"
	"naevis/mq"
	"net/http"
	_ "net/http/pprof"
	"os"
	"path/filepath"

	"mime/multipart"

	"slices"

	"github.com/disintegration/imaging"
	"github.com/julienschmidt/httprouter"
)

func CSRF(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	fmt.Fprint(w, GenerateName(8))
}

func GenerateName(n int) string {
	var letters = []rune("abcdefghijklmnopqrstuvwxyz0123456789_ABCDEFGHIJKLMNOPQRSTUVWXYZ")

	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rndm.Intn(len(letters))]
	}
	return string(b)
}

func GenerateID(n int) string {
	var letters = []rune("abcdefghijklmnopqrstuvwxyz0123456789_ABCDEFGHIJKLMNOPQRSTUVWXYZ")

	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rndm.Intn(len(letters))]
	}
	return string(b)
}

func EncrypIt(strToHash string) string {
	data := []byte(strToHash)
	return fmt.Sprintf("%x", md5.Sum(data))
}

func SendResponse(w http.ResponseWriter, status int, data any, message string, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	response := map[string]any{
		"status":  status,
		"message": message,
		"data":    data,
	}

	if err != nil {
		response["error"] = err.Error()
	}

	// Encode response and check for encoding errors
	if encodeErr := json.NewEncoder(w).Encode(response); encodeErr != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// Helper function to check if a user is in a slice of followers
func Contains(slice []string, value string) bool {
	return slices.Contains(slice, value)
}

// Utility function to send JSON response
func SendJSONResponse(w http.ResponseWriter, status int, response any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// List of supported image MIME types
var SupportedImageTypes = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
	"image/webp": true,
	"image/gif":  true,
	"image/bmp":  true,
	"image/tiff": true,
}

func ValidateImageFileType(w http.ResponseWriter, header *multipart.FileHeader) bool {
	mimeType := header.Header.Get("Content-Type")

	if !SupportedImageTypes[mimeType] {
		http.Error(w, "Invalid file type. Supported formats: JPEG, PNG, WebP, GIF, BMP, TIFF, SVG.", http.StatusBadRequest)
		return false
	}

	return true
}

func CreateThumb(filename string, fileLocation string, fileType string, thumbWidth int, thumbHeight int) error {
	inputPath := filepath.Join(fileLocation, filename+fileType)
	outputDir := filepath.Join(fileLocation, "thumb")
	outputPath := filepath.Join(outputDir, filename+fileType)

	fmt.Println(inputPath)
	fmt.Println(outputPath)

	// Ensure the output directory exists
	if err := ensureDir(outputDir); err != nil {
		log.Printf("failed to create thumbnail directory: %v", err)
		return err
	}

	bgColor := color.Transparent // Use Transparent if preferred

	// Open the original image
	img, err := imaging.Open(inputPath)
	if err != nil {
		log.Printf("failed to open input image: %v", err)
		return err
	}

	// Calculate resized dimensions
	newWidth, newHeight := fitResolution(img.Bounds().Dx(), img.Bounds().Dy(), thumbWidth, thumbHeight)
	resizedImg := imaging.Resize(img, newWidth, newHeight, imaging.Lanczos)

	// Center it in a background canvas
	thumbImg := imaging.New(thumbWidth, thumbHeight, bgColor)
	xPos := (thumbWidth - newWidth) / 2
	yPos := (thumbHeight - newHeight) / 2
	thumbImg = imaging.Paste(thumbImg, resizedImg, image.Pt(xPos, yPos))

	// Save thumbnail
	if err := imaging.Save(thumbImg, outputPath); err != nil {
		log.Printf("failed to save thumbnail: %v", err)
		return err
	}

	// Emit event
	m := mq.Index{}
	mq.Notify("thumbnail-created", m)

	return nil
}

// func CreateThumb(filename string, fileLocation string, fileType string, thumbWidth int, thumbHeight int) error {
// 	// inputPath := fmt.Sprintf("%s/%s%s", fileLocation, filename, fileType)
// 	// outputPath := fmt.Sprintf("%s/thumb/%s%s", fileLocation, filename, fileType)

// 	inputPath := filepath.Join(fileLocation, filename+fileType)
// 	outputPath := filepath.Join(fileLocation, filename+fileType)

// 	// Ensure directory exists
// 	if err := ensureDir(fileLocation); err != nil {
// 		log.Println("failed to create upload directory: %w", err)
// 	}

// 	fmt.Println(outputPath)
// 	// thumbWidth := 300
// 	// thumbHeight := 200
// 	bgColor := color.White // Change to color.Transparent for a transparent background

// 	// Open the original image
// 	img, err := imaging.Open(inputPath)
// 	if err != nil {
// 		return err
// 	}

// 	// Get the original dimensions
// 	origWidth := img.Bounds().Dx()
// 	origHeight := img.Bounds().Dy()

// 	// Calculate new size while maintaining aspect ratio
// 	newWidth, newHeight := fitResolution(origWidth, origHeight, thumbWidth, thumbHeight)

// 	// Resize the image
// 	resizedImg := imaging.Resize(img, newWidth, newHeight, imaging.Lanczos)

// 	// Create a new blank image with the target thumbnail size and a background color
// 	thumbImg := imaging.New(thumbWidth, thumbHeight, bgColor)

// 	// Calculate the position to center the resized image
// 	xPos := (thumbWidth - newWidth) / 2
// 	yPos := (thumbHeight - newHeight) / 2

// 	// Paste the resized image onto the blank canvas
// 	thumbImg = imaging.Paste(thumbImg, resizedImg, image.Pt(xPos, yPos))

// 	// Notify MQ system
// 	m := mq.Index{}
// 	mq.Notify("thumbnail-created", m)

// 	// Save the final thumbnail
// 	return imaging.Save(thumbImg, outputPath)
// }

func fitResolution(origWidth, origHeight, maxWidth, maxHeight int) (int, int) {
	// If the original image is already smaller than the target size, keep it unchanged
	if origWidth <= maxWidth && origHeight <= maxHeight {
		return origWidth, origHeight
	}

	// Calculate the scaling factor for both width and height
	widthRatio := float64(maxWidth) / float64(origWidth)
	heightRatio := float64(maxHeight) / float64(origHeight)

	// Use the smaller ratio to ensure the image fits within bounds
	scaleFactor := math.Min(widthRatio, heightRatio)

	// Compute new dimensions
	newWidth := int(float64(origWidth) * scaleFactor)
	newHeight := int(float64(origHeight) * scaleFactor)

	return newWidth, newHeight
}

// Generic function to ensure directory existence
func ensureDir(dir string) error {
	return os.MkdirAll(dir, 0755)
}

// func CreateThumb(filename string, fileLocation string, fileType string) error {
// 	var inputPath string = fmt.Sprintf("%s/%s%s", fileLocation, filename, fileType)
// 	var fileloc string = fmt.Sprintf("%s/thumb/%s%s", fileLocation, filename, fileType)

// 	fmt.Println(fileloc)
// 	width := 300
// 	height := 200

// 	img, err := imaging.Open(inputPath)
// 	if err != nil {
// 		return err
// 	}
// 	resizedImg := imaging.Resize(img, width, height, imaging.Lanczos)

// 	m := mq.Index{}
// 	mq.Notify("thumbnail-created", m)

// 	return imaging.Save(resizedImg, fileloc)
// }

func GenerateStringName(n int) string {
	var letters = []rune("abcdefghijklmnopqrstuvwxyz0123456789_ABCDEFGHIJKLMNOPQRSTUVWXYZ")

	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rndm.Intn(len(letters))]
	}
	return string(b)
}

func GenerateIntID(n int) string {
	var letters = []rune("0123456789")

	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rndm.Intn(len(letters))]
	}
	return string(b)
}

func GenerateChatID() string {
	// chatIDCounter++
	// return chatIDCounter
	return GenerateIntID(16)
}
