package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/crypto/bcrypt"
)

const (
	refreshTokenTTL = 7 * 24 * time.Hour // 7 days
	accessTokenTTL  = 15 * time.Minute   // 15 minutes
)

var (
	// tokenSigningAlgo = jwt.SigningMethodHS256
	jwtSecret = []byte("your_secret_key") // Replace with a secure secret key
)

func loginHandler(w http.ResponseWriter, r *http.Request) {
	var user User
	if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
		http.Error(w, "Invalid input", http.StatusBadRequest)
		return
	}

	// // Check Redis cache for token
	// cachedToken, err := RdxHget("tokki", user.UserID)
	// if err != nil {
	// 	// Handle Redis error if necessary, log it or silently move on
	// 	log.Printf("Error checking token in Redis: %v", err)
	// }
	// if cachedToken != "" {
	// 	// Token found in cache, return it
	// 	sendResponse(w, http.StatusOK, map[string]string{"token": cachedToken, "userid": user.UserID}, "Login successful", nil)
	// 	return
	// }

	// Look for the user in MongoDB by username
	var storedUser User
	err := userCollection.FindOne(context.TODO(), bson.M{"username": user.Username}).Decode(&storedUser)
	if err != nil {
		// Return generic error message for security reasons
		http.Error(w, "Invalid username or password", http.StatusUnauthorized)
		return
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(storedUser.Password), []byte(user.Password)); err != nil {
		http.Error(w, "Invalid username or password", http.StatusUnauthorized)
		return
	}
	// In login function, after verifying password
	// Remove any existing token for this user in Redis
	_, err = RdxHdel("tokki", storedUser.UserID)
	if err != nil {
		log.Printf("Error removing existing token from Redis: %v", err)
	}

	// Create JWT claims
	claims := &Claims{
		Username: storedUser.Username,
		UserID:   storedUser.UserID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(72 * time.Hour)), // Adjust expiration as needed
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(jwtSecret)
	if err != nil {
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}

	refreshToken, err := generateRefreshToken()
	if err != nil {
		http.Error(w, "Error generating refresh token", http.StatusInternalServerError)
		return
	}

	collection := client.Database("eventdb").Collection("users")
	// Hash the refresh token
	hashedRefreshToken := hashToken(refreshToken)
	_, err = collection.UpdateOne(
		context.TODO(),
		bson.M{"userid": storedUser.UserID},
		bson.M{"$set": bson.M{"refresh_token": hashedRefreshToken, "refresh_expiry": time.Now().Add(refreshTokenTTL)}},
	)
	if err != nil {
		http.Error(w, "Error storing refresh token", http.StatusInternalServerError)
		return
	}

	// Cache the token in Redis (only cache if login is successful)
	err = RdxHset("tokki", claims.UserID, tokenString)
	if err != nil {
		// Log the Redis caching failure, but allow the login to proceed
		log.Printf("Error caching token in Redis: %v", err)
	}

	// Send response with the token
	sendResponse(w, http.StatusOK, map[string]string{"token": tokenString, "refreshToken": refreshToken, "userid": storedUser.UserID}, "Login successful", nil)
}

func registerHandler(w http.ResponseWriter, r *http.Request) {
	var user User
	if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
		http.Error(w, "Invalid input", http.StatusBadRequest)
		return
	}
	log.Printf("Registering user: %s", user.Username)

	// User not found in Redis, check the database
	var existingUser User
	err := userCollection.FindOne(context.TODO(), bson.M{"username": user.Username}).Decode(&existingUser)
	if err == nil {
		// User already exists in database
		log.Printf("User already exists (in DB): %s", user.Username)
		http.Error(w, "User already exists", http.StatusConflict)
		return
	} else if err != mongo.ErrNoDocuments {
		// Handle unexpected error from MongoDB
		log.Printf("Error checking for existing user in DB: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	_, err = RdxHdel("tokki", user.UserID)
	if err != nil {
		log.Printf("Error removing existing token from Redis: %v", err)
	}

	// Proceed with password hashing if user does not already exist
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("Failed to hash password for user %s: %v", user.Username, err)
		http.Error(w, "Failed to hash password", http.StatusInternalServerError)
		return
	}
	user.Password = string(hashedPassword)
	user.UserID = "u" + GenerateName(10)

	// Insert new user into the database
	_, err = userCollection.InsertOne(context.TODO(), user)
	if err != nil {
		log.Printf("Failed to insert user into DB: %v", err)
		http.Error(w, "Failed to register user", http.StatusInternalServerError)
		return
	}

	// Cache the user information in Redis (optional for fast future access)
	err = RdxHset("users", user.UserID, user.Username)
	if err != nil {
		log.Printf("Error caching user in Redis: %v", err)
	}

	// go CreatePreferences(w, r, ps)

	initializeUserDefaults(user.UserID)

	// Respond with success
	w.WriteHeader(http.StatusCreated)
	response := map[string]interface{}{
		"status":  http.StatusCreated,
		"message": "User registered successfully",
		"data":    user.Username,
	}
	json.NewEncoder(w).Encode(response)
}

func logoutUserHandler(w http.ResponseWriter, r *http.Request) {
	tokenString := r.Header.Get("Authorization")
	if tokenString == "" {
		http.Error(w, "Missing token", http.StatusUnauthorized)
		return
	}

	if len(tokenString) < 7 || tokenString[:7] != "Bearer " {
		http.Error(w, "Invalid token format", http.StatusUnauthorized)
		return
	}

	// Extract the token and invalidate it in Redis
	tokenString = tokenString[7:]
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		return jwtSecret, nil
	})

	if err != nil || !token.Valid {
		http.Error(w, "Invalid token", http.StatusUnauthorized)
		return
	}

	// Remove token from Redis cache
	_, err = RdxHdel("tokki", claims.UserID)
	if err != nil {
		log.Printf("Error removing token from Redis: %v", err)
		http.Error(w, "Failed to log out", http.StatusInternalServerError)
		return
	}

	sendResponse(w, http.StatusOK, nil, "User logged out successfully", nil)
}

func refreshTokenHandler(w http.ResponseWriter, r *http.Request) {
	tokenString := r.Header.Get("Authorization")
	if tokenString == "" {
		http.Error(w, "Missing token", http.StatusUnauthorized)
		return
	}

	if len(tokenString) < 7 || tokenString[:7] != "Bearer " {
		http.Error(w, "Invalid token format", http.StatusUnauthorized)
		return
	}

	tokenString = tokenString[7:]
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		return jwtSecret, nil
	})

	if err != nil || !token.Valid {
		http.Error(w, "Invalid token", http.StatusUnauthorized)
		return
	}

	// Ensure the token is not expired and refresh it
	if time.Until(claims.ExpiresAt.Time) < 30*time.Minute {
		claims.ExpiresAt = jwt.NewNumericDate(time.Now().Add(72 * time.Hour)) // Extend the expiration
		newToken := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		newTokenString, err := newToken.SignedString(jwtSecret)
		if err != nil {
			http.Error(w, "Failed to refresh token", http.StatusInternalServerError)
			return
		}

		// Update the token in Redis
		err = RdxHset("tokki", claims.UserID, newTokenString)
		if err != nil {
			log.Printf("Error updating token in Redis: %v", err)
		}

		sendResponse(w, http.StatusOK, map[string]string{"token": newTokenString}, "Token refreshed successfully", nil)
	} else {
		http.Error(w, "Token refresh not allowed yet", http.StatusForbidden)
	}
}

// Generates a random refresh token
func generateRefreshToken() (string, error) {
	tokenBytes := make([]byte, 32)
	_, err := rand.Read(tokenBytes)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(tokenBytes), nil
}

// Hashes a given token
func hashToken(token string) string {
	hash := sha256.New()
	hash.Write([]byte(token))
	return hex.EncodeToString(hash.Sum(nil))
}
