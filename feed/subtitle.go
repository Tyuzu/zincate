package feed

import (
	"fmt"
	"os"
)

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
