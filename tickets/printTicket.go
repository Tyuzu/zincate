package tickets

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"log"
	"naevis/db"
	"naevis/profile"
	"naevis/structs"
	"net/http"
	_ "net/http/pprof"
	"time"

	"github.com/julienschmidt/httprouter"
	"github.com/phpdave11/gofpdf"
	"github.com/skip2/go-qrcode"
	"go.mongodb.org/mongo-driver/bson"
)

const hmacSecret = "your-very-secret-key" // keep secure

// GenerateQRPayload returns a secure payload string: eventID|ticketID|uniqueCode|timestamp|signature
func GenerateQRPayload(eventID, ticketID, uniqueCode string) string {
	timestamp := time.Now().Unix() // current UNIX timestamp
	data := fmt.Sprintf("%s|%s|%s|%d", eventID, ticketID, uniqueCode, timestamp)

	h := hmac.New(sha256.New, []byte(hmacSecret))
	h.Write([]byte(data))
	sig := base64.StdEncoding.EncodeToString(h.Sum(nil))

	return fmt.Sprintf("%s|%s", data, sig)
}

// func PrintTicket(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
// 	eventID := ps.ByName("eventid")
// 	ticketID := ps.ByName("ticketid")
// 	uniqueCode := r.URL.Query().Get("uniqueCode")

// 	tokenString := r.Header.Get("Authorization")
// 	claims, err := profile.ValidateJWT(tokenString)
// 	if err != nil {
// 		log.Printf("JWT validation error: %v", err)
// 		http.Error(w, "Unauthorized", http.StatusUnauthorized)
// 		return
// 	}

// 	if uniqueCode == "" {
// 		http.Error(w, "Unique code is required for verification", http.StatusBadRequest)
// 		return
// 	}

// 	var purchasedTicket structs.PurchasedTicket
// 	err = db.PurchasedTicketsCollection.FindOne(context.TODO(), bson.M{
// 		"eventid":    eventID,
// 		"ticketid":   ticketID,
// 		"uniquecode": uniqueCode,
// 	}).Decode(&purchasedTicket)
// 	if err != nil {
// 		http.Error(w, fmt.Sprintf("Ticket verification failed: %v", err), http.StatusNotFound)
// 		return
// 	}

// 	purchasedTicket.BuyerName = claims.Username

// 	// Construct ticket payload
// 	ticketData := fmt.Sprintf("%s|%s|%s", eventID, ticketID, uniqueCode)
// 	h := hmac.New(sha256.New, []byte(hmacSecret))
// 	h.Write([]byte(ticketData))
// 	signature := base64.StdEncoding.EncodeToString(h.Sum(nil))

// 	qrPayload := fmt.Sprintf("%s|%s", ticketData, signature)

// 	// Generate QR code
// 	qrPNG, err := qrcode.Encode(qrPayload, qrcode.Medium, 256)
// 	if err != nil {
// 		http.Error(w, "Failed to generate QR code", http.StatusInternalServerError)
// 		return
// 	}

// 	// Create PDF
// 	pdf := gofpdf.New("P", "mm", "A4", "")
// 	pdf.AddPage()
// 	pdf.SetFont("Arial", "B", 16)
// 	pdf.Cell(40, 10, "Event Ticket")
// 	pdf.Ln(12)

// 	pdf.SetFont("Arial", "", 12)
// 	pdf.Cell(0, 10, fmt.Sprintf("Event ID: %s", eventID))
// 	pdf.Ln(8)
// 	pdf.Cell(0, 10, fmt.Sprintf("Ticket ID: %s", ticketID))
// 	pdf.Ln(8)
// 	pdf.Cell(0, 10, fmt.Sprintf("Name: %s", purchasedTicket.BuyerName))
// 	pdf.Ln(8)
// 	pdf.Cell(0, 10, fmt.Sprintf("Unique Code: %s", uniqueCode))
// 	pdf.Ln(12)

// 	// Add QR image
// 	imageOpts := gofpdf.ImageOptions{
// 		ImageType: "PNG",
// 	}
// 	pdf.RegisterImageOptionsReader("qr", imageOpts, bytes.NewReader(qrPNG))
// 	pdf.ImageOptions("qr", 150, 40, 40, 40, false, imageOpts, 0, "")

// 	var buf bytes.Buffer
// 	if err := pdf.Output(&buf); err != nil {
// 		http.Error(w, "Failed to generate PDF", http.StatusInternalServerError)
// 		return
// 	}

// 	w.Header().Set("Content-Type", "application/pdf")
// 	w.Header().Set("Content-Disposition", "attachment; filename=ticket-"+ticketID+".pdf")
// 	w.WriteHeader(http.StatusOK)
// 	w.Write(buf.Bytes())
// }

func PrintTicket(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	eventID := ps.ByName("eventid")
	uniqueCode := r.URL.Query().Get("uniqueCode")

	tokenString := r.Header.Get("Authorization")
	claims, err := profile.ValidateJWT(tokenString)
	if err != nil {
		log.Printf("JWT validation error: %v", err)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if uniqueCode == "" {
		http.Error(w, "Unique code is required for verification", http.StatusBadRequest)
		return
	}

	var purchasedTicket structs.PurchasedTicket
	err = db.PurchasedTicketsCollection.FindOne(context.TODO(), bson.M{
		"eventid":    eventID,
		"uniquecode": uniqueCode,
	}).Decode(&purchasedTicket)
	if err != nil {
		http.Error(w, fmt.Sprintf("Ticket verification failed: %v", err), http.StatusNotFound)
		return
	}

	purchasedTicket.BuyerName = claims.Username

	// Construct ticket payload
	ticketData := fmt.Sprintf("%s|%s", eventID, uniqueCode)
	h := hmac.New(sha256.New, []byte(hmacSecret))
	h.Write([]byte(ticketData))
	signature := base64.StdEncoding.EncodeToString(h.Sum(nil))

	qrPayload := fmt.Sprintf("%s|%s", ticketData, signature)

	// Generate QR code
	qrPNG, err := qrcode.Encode(qrPayload, qrcode.Medium, 256)
	if err != nil {
		http.Error(w, "Failed to generate QR code", http.StatusInternalServerError)
		return
	}

	// Create PDF
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.AddPage()
	pdf.SetFont("Arial", "B", 16)
	pdf.Cell(40, 10, "Event Ticket")
	pdf.Ln(12)

	pdf.SetFont("Arial", "", 12)
	pdf.Cell(0, 10, fmt.Sprintf("Event ID: %s", eventID))
	pdf.Ln(8)
	pdf.Cell(0, 10, fmt.Sprintf("Name: %s", purchasedTicket.BuyerName))
	pdf.Ln(8)
	pdf.Cell(0, 10, fmt.Sprintf("Unique Code: %s", uniqueCode))
	pdf.Ln(12)

	// Add QR image
	imageOpts := gofpdf.ImageOptions{
		ImageType: "PNG",
	}
	pdf.RegisterImageOptionsReader("qr", imageOpts, bytes.NewReader(qrPNG))
	pdf.ImageOptions("qr", 150, 40, 40, 40, false, imageOpts, 0, "")

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		http.Error(w, "Failed to generate PDF", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", "attachment; filename=ticket-"+uniqueCode+".pdf")
	w.WriteHeader(http.StatusOK)
	w.Write(buf.Bytes())
}

// import (
// 	"bytes"
// 	"context"
// 	"fmt"
// 	"log"
// 	"naevis/db"
// 	"naevis/profile"
// 	"naevis/structs"
// 	"net/http"

// 	"github.com/julienschmidt/httprouter"
// 	"github.com/phpdave11/gofpdf"
// 	"go.mongodb.org/mongo-driver/bson"
// )

// func PrintTicket(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
// 	eventID := ps.ByName("eventid")
// 	ticketID := ps.ByName("ticketid")
// 	uniqueCode := r.URL.Query().Get("uniqueCode")

// 	tokenString := r.Header.Get("Authorization")
// 	claims, err := profile.ValidateJWT(tokenString)
// 	if err != nil {
// 		log.Printf("JWT validation error: %v", err) // Log the error for debugging
// 		http.Error(w, "Unauthorized", http.StatusUnauthorized)
// 		return
// 	}

// 	if uniqueCode == "" {
// 		http.Error(w, "Unique code is required for verification", http.StatusBadRequest)
// 		return
// 	}

// 	var purchasedTicket structs.PurchasedTicket
// 	err = db.PurchasedTicketsCollection.FindOne(context.TODO(), bson.M{
// 		"eventid":    eventID,
// 		"ticketid":   ticketID,
// 		"uniquecode": uniqueCode,
// 	}).Decode(&purchasedTicket)
// 	if err != nil {
// 		http.Error(w, fmt.Sprintf("Ticket verification failed: %v", err), http.StatusNotFound)
// 		return
// 	}

// 	purchasedTicket.BuyerName = claims.Username

// 	// Generate PDF
// 	pdf := gofpdf.New("P", "mm", "A4", "")
// 	pdf.AddPage()
// 	pdf.SetFont("Arial", "B", 16)
// 	pdf.Cell(40, 10, "Event Ticket")
// 	pdf.Ln(10)
// 	pdf.SetFont("Arial", "", 12)
// 	pdf.Cell(40, 10, fmt.Sprintf("Event ID: %s", eventID))
// 	pdf.Ln(8)
// 	pdf.Cell(40, 10, fmt.Sprintf("Ticket ID: %s", ticketID))
// 	pdf.Ln(8)
// 	pdf.Cell(40, 10, fmt.Sprintf("Unique Code: %s", uniqueCode))
// 	pdf.Ln(8)
// 	pdf.Cell(40, 10, fmt.Sprintf("Name: %s", purchasedTicket.BuyerName)) // Adjust field as needed

// 	var buf bytes.Buffer
// 	err = pdf.Output(&buf)
// 	if err != nil {
// 		http.Error(w, "Failed to generate PDF", http.StatusInternalServerError)
// 		return
// 	}

// 	// Serve the PDF
// 	w.Header().Set("Content-Type", "application/pdf")
// 	w.Header().Set("Content-Disposition", "attachment; filename=ticket-"+ticketID+".pdf")
// 	w.WriteHeader(http.StatusOK)
// 	w.Write(buf.Bytes())
// }
