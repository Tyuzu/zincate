package chathandlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"naevis/db"
	"naevis/middleware"
	"naevis/structs"
	"naevis/utils"
	"naevis/websock"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// --- Utility Functions ---

func GetChatMessages(chatID string, limit int64) ([]structs.Message, error) {
	filter := bson.M{"chat_id": chatID, "deleted": false}
	opts := options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}}).SetLimit(limit)

	cur, err := db.MessagesCollection.Find(structs.Ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cur.Close(structs.Ctx)

	var msgs []structs.Message
	for cur.Next(structs.Ctx) {
		var msg structs.Message
		if err := cur.Decode(&msg); err != nil {
			log.Println("Decode message error:", err)
			continue
		}
		msgs = append(msgs, msg)
	}

	if len(msgs) == 0 {
		msgs = []structs.Message{}
	}

	return msgs, nil
}

func saveMessage(msg structs.Message) error {
	_, err := db.MessagesCollection.InsertOne(structs.Ctx, msg)
	return err
}

func updateMessage(chatID, messageID string, update bson.M) error {
	filter := bson.M{"chat_id": chatID, "message_id": messageID}
	_, err := db.MessagesCollection.UpdateOne(structs.Ctx, filter, bson.M{"$set": update})
	return err
}

func getMessage(chatID, messageID string) (*structs.Message, error) {
	// Define the filter for finding the specific message
	filter := bson.M{"chat_id": chatID, "message_id": messageID}

	var message structs.Message
	err := db.MessagesCollection.FindOne(context.TODO(), filter).Decode(&message)
	if err != nil {
		return nil, err
	}

	return &message, nil
}

// --- Handlers ---

// Fetch messages from MongoDB
func MessagesHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	tokenString := r.Header.Get("Authorization")
	claims, err := middleware.ValidateJWT(tokenString)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	_ = claims

	chatID := r.URL.Query().Get("chat_id")
	if chatID == "" {
		http.Error(w, "chat_id is required", http.StatusBadRequest)
		return
	}

	messages, err := GetChatMessages(chatID, 20)
	if err != nil {
		http.Error(w, "Failed to fetch messages", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(messages)
}

// func SendMessageHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
// 	tokenString := r.Header.Get("Authorization")
// 	claims, err := middleware.ValidateJWT(tokenString)
// 	if err != nil {
// 		http.Error(w, "Unauthorized", http.StatusUnauthorized)
// 		return
// 	}

// 	if r.Method != http.MethodPost {
// 		http.Error(w, "Invalid Method", http.StatusMethodNotAllowed)
// 		return
// 	}

// 	err = r.ParseMultipartForm(20 << 20) // 20MB limit
// 	if err != nil {
// 		http.Error(w, "Failed to parse form: "+err.Error(), http.StatusBadRequest)
// 		return
// 	}

// 	chatID := r.FormValue("chat_id")
// 	content := r.FormValue("content")
// 	caption := r.FormValue("caption")

// 	if chatID == "" {
// 		http.Error(w, "chat_id is required", http.StatusBadRequest)
// 		return
// 	}

// 	files := r.MultipartForm.File["files"]
// 	if len(files) > 6 {
// 		http.Error(w, "Too many files (max 6 allowed)", http.StatusBadRequest)
// 		return
// 	}

// 	var filenames []string
// 	var filetype []string

// 	for _, fileHeader := range files {
// 		if fileHeader.Size > 8*1024*1024 {
// 			http.Error(w, "One or more files exceed 8MB size limit", http.StatusBadRequest)
// 			return
// 		}

// 		file, err := fileHeader.Open()
// 		if err != nil {
// 			http.Error(w, "Failed to open file: "+err.Error(), http.StatusInternalServerError)
// 			return
// 		}

// 		fileName := utils.GenerateIntID(16)
// 		log.Println("Saving file:", fileName)

// 		extn, _ := getFileType(fileHeader.Filename)
// 		filetype = append(filetype, extn)

// 		if err := saveUploadedFile(file, fileName, extn); err != nil {
// 			file.Close()
// 			http.Error(w, "Failed to save file: "+err.Error(), http.StatusInternalServerError)
// 			return
// 		}
// 		file.Close()

// 		filenames = append(filenames, fileName)

// 		// // if i == 0 {
// 		// ft, _ := getFileType(fileHeader.Filename)
// 		// filetype = append(filetype, ft)
// 		// // }
// 	}

// 	msg := structs.Message{
// 		MessageID: generateMessageID(),
// 		ChatID:    chatID,
// 		Content:   content,
// 		Caption:   caption,
// 		File:      filenames,
// 		FileType:  filetype,
// 		SenderID:  claims.UserID,
// 		CreatedAt: time.Now(),
// 		EditedAt:  time.Now(),
// 	}

// 	if err := saveMessage(msg); err != nil {
// 		http.Error(w, "Failed to save message", http.StatusInternalServerError)
// 		return
// 	}

// 	w.Header().Set("Content-Type", "application/json")
// 	json.NewEncoder(w).Encode(msg)
// }

// func getFileType(filename string) (string, error) {
// 	// file, err := os.Open(filename)
// 	// if err != nil {
// 	// 	return "", err
// 	// }
// 	// defer file.Close()

// 	// buffer := make([]byte, 512)
// 	// _, err = file.Read(buffer)

// 	// if err != nil {
// 	// 	return "", err
// 	// }

//		fileType := ""
//		extension := filepath.Ext(filename)
//		if extension != "" {
//			fileType = extension[1:]
//		}
//		return fileType, nil
//	}
func SendMessageHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid Method", http.StatusMethodNotAllowed)
		return
	}

	claims, err := validateAuth(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	formData, err := parseMessageForm(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	filenames, filetypes, err := processUploadedFiles(formData.Files)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	msg := structs.Message{
		MessageID: generateMessageID(),
		ChatID:    formData.ChatID,
		Content:   formData.Content,
		Caption:   formData.Caption,
		File:      filenames,
		FileType:  filetypes,
		SenderID:  claims.UserID,
		CreatedAt: time.Now(),
		EditedAt:  time.Now(),
	}

	if err := saveMessage(msg); err != nil {
		http.Error(w, "Failed to save message", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(msg)
}

type MessageFormData struct {
	ChatID  string
	Content string
	Caption string
	Files   []*multipart.FileHeader
}

func validateAuth(r *http.Request) (*middleware.Claims, error) {
	tokenString := r.Header.Get("Authorization")
	return middleware.ValidateJWT(tokenString)
}

func parseMessageForm(r *http.Request) (*MessageFormData, error) {
	if err := r.ParseMultipartForm(20 << 20); err != nil {
		return nil, fmt.Errorf("failed to parse form: %v", err)
	}

	chatID := r.FormValue("chat_id")
	if chatID == "" {
		return nil, fmt.Errorf("chat_id is required")
	}

	files := r.MultipartForm.File["files"]
	if len(files) > 6 {
		return nil, fmt.Errorf("too many files (max 6 allowed)")
	}

	return &MessageFormData{
		ChatID:  chatID,
		Content: r.FormValue("content"),
		Caption: r.FormValue("caption"),
		Files:   files,
	}, nil
}

func processUploadedFiles(files []*multipart.FileHeader) ([]string, []string, error) {
	var filenames []string
	var filetypes []string

	for _, fileHeader := range files {
		if fileHeader.Size > 8*1024*1024 {
			return nil, nil, fmt.Errorf("one or more files exceed 8MB size limit")
		}

		file, err := fileHeader.Open()
		if err != nil {
			return nil, nil, fmt.Errorf("failed to open file: %v", err)
		}
		defer file.Close()

		filename := utils.GenerateIntID(16)
		extn, _ := getFileType(fileHeader.Filename)
		if err := saveUploadedFile(file, filename, extn); err != nil {
			return nil, nil, fmt.Errorf("failed to save file: %v", err)
		}

		filenames = append(filenames, filename)
		filetypes = append(filetypes, extn)
	}

	return filenames, filetypes, nil
}

func getFileType(filename string) (string, error) {
	ext := filepath.Ext(filename)
	if ext == "" {
		return "", fmt.Errorf("missing file extension")
	}
	return strings.TrimPrefix(ext, "."), nil
}

func EditMessageHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	tokenString := r.Header.Get("Authorization")
	claims, err := middleware.ValidateJWT(tokenString)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if r.Method != http.MethodPut {
		http.Error(w, "Invalid Method", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ChatID     string `json:"chat_id"`
		MessageID  string `json:"message_id"`
		NewContent string `json:"new_content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request data", http.StatusBadRequest)
		return
	}

	// Fetch the message to verify the sender
	message, err := getMessage(req.ChatID, req.MessageID)
	if err != nil {
		http.Error(w, "Message not found", http.StatusNotFound)
		return
	}

	// Check if the user is the sender of the message
	if claims.UserID != message.SenderID {
		http.Error(w, "Forbidden: Only the sender can edit the message", http.StatusForbidden)
		return
	}

	update := bson.M{
		"content":   req.NewContent,
		"edited_at": time.Now(),
	}

	if err := updateMessage(req.ChatID, req.MessageID, update); err != nil {
		http.Error(w, "Failed to update message", http.StatusInternalServerError)
		return
	}

	wsMessage := struct {
		Type       string `json:"type"`
		ChatID     string `json:"chat_id"`
		MessageID  string `json:"message_id"`
		NewContent string `json:"new_content"`
	}{
		Type:       "edit",
		ChatID:     req.ChatID,
		MessageID:  req.MessageID,
		NewContent: req.NewContent,
	}
	websock.WsBroadcast(req.ChatID, wsMessage)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(update)
}

// // Delete a message (soft delete)
// func DeleteMessageHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
// 	tokenString := r.Header.Get("Authorization")
// 	claims, err := middleware.ValidateJWT(tokenString)
// 	if err != nil {
// 		http.Error(w, "Unauthorized", http.StatusUnauthorized)
// 		return
// 	}
// 	_ = claims.UserID

// 	if r.Method != http.MethodDelete {
// 		http.Error(w, "Invalid Method", http.StatusMethodNotAllowed)
// 		return
// 	}

// 	var req struct {
// 		ChatID    string `json:"chat_id"`
// 		MessageID string `json:"message_id"`
// 	}
// 	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
// 		http.Error(w, "Invalid request data", http.StatusBadRequest)
// 		return
// 	}

// 	update := bson.M{"deleted": true}

// 	if err := updateMessage(req.ChatID, req.MessageID, update); err != nil {
// 		http.Error(w, "Failed to delete message", http.StatusInternalServerError)
// 		return
// 	}

// 	// wsMessage := struct {
// 	// 	Type      string `json:"type"`
// 	// 	ChatID    string `json:"chat_id"`
// 	// 	MessageID string `json:"message_id"`
// 	// }{
// 	// 	Type:      "delete",
// 	// 	ChatID:    req.ChatID,
// 	// 	MessageID: req.MessageID,
// 	// }
// 	// wsBroadcast(req.ChatID, wsMessage)

// 	// w.WriteHeader(http.StatusNoContent)

// 	w.Header().Set("Content-Type", "application/json")
// 	json.NewEncoder(w).Encode(update)
// }

func DeleteMessageHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	tokenString := r.Header.Get("Authorization")
	claims, err := middleware.ValidateJWT(tokenString)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if r.Method != http.MethodDelete {
		http.Error(w, "Invalid Method", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ChatID    string `json:"chat_id"`
		MessageID string `json:"message_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request data", http.StatusBadRequest)
		return
	}

	// Fetch the message to verify sender
	message, err := getMessage(req.ChatID, req.MessageID)
	if err != nil {
		http.Error(w, "Message not found", http.StatusNotFound)
		return
	}

	// Check if the user is the sender of the message
	if claims.UserID != message.SenderID {
		http.Error(w, "Forbidden: Only the sender can delete the message", http.StatusForbidden)
		return
	}

	update := bson.M{"deleted": true}

	if err := updateMessage(req.ChatID, req.MessageID, update); err != nil {
		http.Error(w, "Failed to delete message", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(update)
}

func saveUploadedFile(src multipart.File, filename, extn string) error {
	defer src.Close()

	fullPath := "./static/chatpic/" + filename + "." + extn
	dst, err := os.Create(fullPath)
	if err != nil {
		return err
	}
	defer dst.Close()

	_, err = io.Copy(dst, src)
	if err != nil {
		return err
	}

	// Check if file is a video and create poster
	if isVideoFile(extn) {
		posterPath := "./static/chatpic/" + filename + ".jpg"
		err = CreatePoster(fullPath, posterPath, "00:00:01") // 1-second timestamp
		if err != nil {
			return fmt.Errorf("failed to create poster: %w", err)
		}
	}

	return nil
}

func isVideoFile(extn string) bool {
	switch extn {
	case "mp4", "mov", "avi", "webm", "mkv":
		return true
	default:
		return false
	}
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

func generateMessageID() string {
	// return fmt.Sprintf("%d", time.Now().UnixNano()) // Replace with a proper unique ID generator
	return utils.GenerateIntID(18)
}
