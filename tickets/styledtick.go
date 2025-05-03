package tickets

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"time"

	"github.com/julienschmidt/httprouter"
	"github.com/phpdave11/gofpdf"
	"github.com/skip2/go-qrcode"
	"go.mongodb.org/mongo-driver/bson"

	"naevis/db"
	"naevis/profile"
	"naevis/structs"
)

func PrintStyledTicket(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	eventID := ps.ByName("eventid")
	ticketID := ps.ByName("ticketid")
	uniqueCode := r.URL.Query().Get("uniqueCode")
	token := r.Header.Get("Authorization")

	// Validate JWT
	claims, err := profile.ValidateJWT(token)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if uniqueCode == "" {
		http.Error(w, "Missing unique code", http.StatusBadRequest)
		return
	}

	// Fetch ticket
	var ticket structs.PurchasedTicket
	err = db.PurchasedTicketsCollection.FindOne(context.TODO(), bson.M{
		"eventid":    eventID,
		"ticketid":   ticketID,
		"uniquecode": uniqueCode,
	}).Decode(&ticket)
	if err != nil {
		http.Error(w, "Ticket not found", http.StatusNotFound)
		return
	}

	ticket.BuyerName = claims.Username
	timestamp := time.Now().Unix()

	// Generate QR content with timestamp
	qrData := fmt.Sprintf("eid=%s&tid=%s&code=%s&ts=%d", eventID, ticketID, uniqueCode, timestamp)
	qrCode, _ := qrcode.Encode(qrData, qrcode.Medium, 128)

	// Generate PDF
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(20, 20, 20)
	pdf.AddPage()
	pdf.SetFillColor(245, 245, 255)

	// Title
	pdf.SetFont("Arial", "B", 20)
	pdf.CellFormat(0, 15, "🎫 Your Event Ticket", "", 1, "C", false, 0, "")
	pdf.Ln(5)

	// Buyer Info
	pdf.SetFont("Arial", "", 12)
	pdf.MultiCell(0, 10, fmt.Sprintf(
		"Name: %s\nEvent ID: %s\nTicket ID: %s\nIssued: %s",
		ticket.BuyerName,
		eventID,
		ticketID,
		time.Now().Format("02 Jan 2006 15:04"),
	), "", "L", false)

	// QR Code Image
	imgOpts := gofpdf.ImageOptions{ImageType: "png"}
	pdf.RegisterImageOptionsReader("qr", imgOpts, bytes.NewReader(qrCode))
	pdf.ImageOptions("qr", 150, 60, 40, 40, false, imgOpts, 0, "")

	// Footer
	pdf.SetY(-30)
	pdf.SetFont("Arial", "I", 10)
	pdf.CellFormat(0, 10, "Show this ticket at entry. Scanners will validate with timestamp.", "T", 0, "C", false, 0, "")

	var buf bytes.Buffer
	err = pdf.Output(&buf)
	if err != nil {
		http.Error(w, "Failed to generate ticket", http.StatusInternalServerError)
		return
	}

	// Send PDF
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", "attachment; filename=ticket-"+ticketID+".pdf")
	w.Write(buf.Bytes())
}

/********/
// package tickets

// import (
// 	"bytes"
// 	"context"
// 	"fmt"
// 	"image/png"
// 	"log"
// 	"net/http"
// 	"time"

// 	"github.com/julienschmidt/httprouter"
// 	"github.com/phpdave11/gofpdf"
// 	"github.com/skip2/go-qrcode"
// 	"go.mongodb.org/mongo-driver/bson"

// 	"naevis/db"
// 	"naevis/profile"
// 	"naevis/structs"
// )

// func PrintStyledTicket(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
// 	eventID := ps.ByName("eventid")
// 	ticketID := ps.ByName("ticketid")
// 	uniqueCode := r.URL.Query().Get("uniqueCode")

// 	tokenString := r.Header.Get("Authorization")
// 	claims, err := profile.ValidateJWT(tokenString)
// 	if err != nil {
// 		http.Error(w, "Unauthorized", http.StatusUnauthorized)
// 		return
// 	}

// 	if uniqueCode == "" {
// 		http.Error(w, "Unique code is required", http.StatusBadRequest)
// 		return
// 	}

// 	// DB query
// 	var purchasedTicket structs.PurchasedTicket
// 	err = db.PurchasedTicketsCollection.FindOne(context.TODO(), bson.M{
// 		"eventid":    eventID,
// 		"ticketid":   ticketID,
// 		"uniquecode": uniqueCode,
// 	}).Decode(&purchasedTicket)
// 	if err != nil {
// 		http.Error(w, "Ticket not found", http.StatusNotFound)
// 		return
// 	}

// 	// Update buyer name from JWT
// 	purchasedTicket.BuyerName = claims.Username

// 	// Generate QR payload
// 	payload := fmt.Sprintf("%s|%s|%s|%d", eventID, ticketID, uniqueCode, time.Now().Unix())

// 	qrPNG, err := qrcode.Encode(payload, qrcode.Medium, 128)
// 	if err != nil {
// 		http.Error(w, "QR generation failed", http.StatusInternalServerError)
// 		return
// 	}
// 	qrImage, err := png.Decode(bytes.NewReader(qrPNG))
// 	if err != nil {
// 		http.Error(w, "QR decode failed", http.StatusInternalServerError)
// 		return
// 	}

// 	// Start PDF
// 	pdf := gofpdf.New("P", "mm", "A4", "")
// 	pdf.SetMargins(15, 15, 15)
// 	pdf.AddPage()

// 	// Colors and fonts
// 	pdf.SetFillColor(240, 248, 255) // light background
// 	pdf.SetDrawColor(100, 100, 255)
// 	pdf.SetLineWidth(0.5)

// 	// Title bar
// 	pdf.SetFont("Arial", "B", 24)
// 	pdf.CellFormat(0, 20, "🎫 Event Ticket", "0", 1, "C", false, 0, "")

// 	pdf.Ln(5)

// 	// Main section box
// 	pdf.SetFont("Arial", "", 12)
// 	pdf.MultiCell(0, 10, fmt.Sprintf(
// 		"Name: %s\nEvent ID: %s\nTicket ID: %s\nUnique Code: %s\nIssue Date: %s",
// 		purchasedTicket.BuyerName,
// 		eventID,
// 		ticketID,
// 		uniqueCode,
// 		time.Now().Format("02 Jan 2006 15:04"),
// 	), "", "L", false)

// 	pdf.Ln(5)

// 	// QR Code on right
// 	imgOpts := gofpdf.ImageOptions{ImageType: "png", ReadDpi: true}
// 	pdf.RegisterImageOptionsReader("qr", imgOpts, bytes.NewReader(qrPNG))
// 	pdf.ImageOptions("qr", 150, 50, 40, 40, false, imgOpts, 0, ""
