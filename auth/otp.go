package auth

import (
	"context"
	"encoding/json"
	"math/rand"
	"naevis/db"
	"naevis/rdx"
	"net/http"
	"net/smtp"
	"strings"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
)

func GenerateOTP(length int) string {
	digits := "0123456789"
	var otp strings.Builder
	for i := 0; i < length; i++ {
		otp.WriteByte(digits[rand.Intn(len(digits))])
	}
	return otp.String()
}

func SendEmailOTP(toEmail, otp string) error {
	from := "youremail@example.com"
	pass := "your-app-password"
	smtpHost := "smtp.gmail.com"
	smtpPort := "587"

	msg := []byte("Subject: Email Verification\n\nYour OTP is: " + otp)

	auth := smtp.PlainAuth("", from, pass, smtpHost)
	return smtp.SendMail(smtpHost+":"+smtpPort, auth, from, []string{toEmail}, msg)
}

func VerifyOTPHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	var input struct {
		Email string `json:"email"`
		OTP   string `json:"otp"`
	}
	json.NewDecoder(r.Body).Decode(&input)

	storedOTP, err := rdx.RdxGet("otp:" + input.Email)
	if err != nil || storedOTP != input.OTP {
		http.Error(w, "Invalid or expired OTP", http.StatusUnauthorized)
		return
	}

	// Mark user as verified
	_, err = db.UserCollection.UpdateOne(
		context.TODO(),
		bson.M{"email": input.Email},
		bson.M{"$set": bson.M{"email_verified": true}},
	)
	if err != nil {
		http.Error(w, "Failed to verify user", http.StatusInternalServerError)
		return
	}

	rdx.RdxDel("otp:" + input.Email) // Clean up OTP
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "User verified successfully"})
}
