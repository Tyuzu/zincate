package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/golang-jwt/jwt/v5"
	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/crypto/bcrypt"
)

// Handlers for user profile

func getUserProfile(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	claims, ok := r.Context().Value(userIDKey).(*Claims)
	if !ok || claims.UserID == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	username := ps.ByName("username")

	// Retrieve user details
	user, err := GetUserByUsername(username)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if user == nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	// // Retrieve follow data
	// userFollow, err := GetUserFollowData(user.UserID)
	// if err != nil {
	// 	http.Error(w, "Internal server error", http.StatusInternalServerError)
	// 	return
	// }

	// Build and respond with the user profile
	userProfile := UserProfileResponse{
		UserID:         user.UserID,
		Username:       user.Username,
		Email:          user.Email,
		Bio:            user.Bio,
		ProfilePicture: user.ProfilePicture,
		BannerPicture:  user.BannerPicture,
		// Followers:      len(userFollow.Followers),
		// Follows:        len(userFollow.Follows),
		// IsFollowing:    contains(userFollow.Followers, claims.UserID),
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(userProfile)
}

func editProfile(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	claims, err := validateJWT(r.Header.Get("Authorization"))
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Parse form data
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		http.Error(w, "Unable to parse form", http.StatusBadRequest)
		return
	}

	// Invalidate cached profile
	_ = InvalidateCachedProfile(claims.Username)

	// Update profile fields
	updates, err := updateProfileFields(w, r, claims)
	if err != nil {
		http.Error(w, "Failed to update profile fields", http.StatusInternalServerError)
		return
	}

	// Save updates to the database
	if err := UpdateUserByUsername(claims.Username, updates); err != nil {
		http.Error(w, "Failed to update profile", http.StatusInternalServerError)
		return
	}

	// Respond with the updated profile
	if err := respondWithUserProfile(w, claims.Username); err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

func getProfile(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	tokenString := r.Header.Get("Authorization")
	claims, err := validateJWT(tokenString)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Check Redis cache
	cachedProfile, err := GetCachedProfile(claims.Username)
	if err == nil && cachedProfile != "" {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(cachedProfile))
		return
	}

	// Retrieve user profile from DB
	user, err := GetUserByUsername(claims.Username)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if user == nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	user.Password = "" // Do not return the password

	// Cache and return profile
	profileJSON, _ := json.Marshal(user)
	_ = CacheProfile(claims.Username, string(profileJSON))

	w.Header().Set("Content-Type", "application/json")
	w.Write(profileJSON)
}

func deleteProfile(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	claims, ok := r.Context().Value(userIDKey).(*Claims)
	if !ok || claims.UserID == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Invalidate cached profile
	_ = InvalidateCachedProfile(claims.Username)

	// Delete profile from DB
	if err := DeleteUserByID(claims.UserID); err != nil {
		http.Error(w, "Failed to delete profile", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Profile deleted successfully"})
}

func updateProfileFields(w http.ResponseWriter, r *http.Request, claims *Claims) (bson.M, error) {
	update := bson.M{}

	// Retrieve and update fields from the form
	if username := r.FormValue("username"); username != "" {
		update["username"] = username
		_ = RdxHset("users", claims.UserID, username)
	}
	if email := r.FormValue("email"); email != "" {
		update["email"] = email
	}
	if bio := r.FormValue("bio"); bio != "" {
		update["bio"] = bio
	}
	if phoneNumber := r.FormValue("phone"); phoneNumber != "" {
		update["phone_number"] = phoneNumber
	}

	// Optional: handle password update
	if password := r.FormValue("password"); password != "" {
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			http.Error(w, "Failed to hash password", http.StatusInternalServerError)
			return nil, err
		}
		update["password"] = string(hashedPassword)
	}

	return update, nil
}

func validateJWT(tokenString string) (*Claims, error) {
	if tokenString == "" || len(tokenString) < 8 {
		return nil, fmt.Errorf("invalid token")
	}

	claims := &Claims{}
	_, err := jwt.ParseWithClaims(tokenString[7:], claims, func(token *jwt.Token) (interface{}, error) {
		return jwtSecret, nil
	})
	if err != nil {
		return nil, fmt.Errorf("unauthorized: %w", err)
	}
	return claims, nil
}

func respondWithUserProfile(w http.ResponseWriter, username string) error {
	var userProfile User
	err := userCollection.FindOne(context.TODO(), bson.M{"username": username}).Decode(&userProfile)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			http.Error(w, "User not found", http.StatusNotFound)
			return nil
		}
		return err
	}

	w.Header().Set("Content-Type", "application/json")
	return json.NewEncoder(w).Encode(userProfile)
}

// Update profile picture
func editProfilePic(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	// Validate JWT token
	claims, err := validateJWT(r.Header.Get("Authorization"))
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Parse the multipart form
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		http.Error(w, "Unable to parse form", http.StatusBadRequest)
		return
	}

	// Update profile picture
	pictureUpdates, err := updateProfilePictures(w, r, claims)
	if err != nil {
		http.Error(w, "Failed to update profile picture", http.StatusInternalServerError)
		return
	}

	// Save updated profile picture to the database
	if err := applyProfileUpdates(claims.Username, pictureUpdates); err != nil {
		http.Error(w, "Failed to update profile picture", http.StatusInternalServerError)
		return
	}

	// Respond with the updated profile
	if err := respondWithUserProfile(w, claims.Username); err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// Update banner picture
func editProfileBanner(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	// Validate JWT token
	claims, err := validateJWT(r.Header.Get("Authorization"))
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Parse the multipart form
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		http.Error(w, "Unable to parse form", http.StatusBadRequest)
		return
	}

	// Update banner picture
	bannerUpdates, err := uploadBannerHandler(w, r, claims)
	if err != nil {
		http.Error(w, "Failed to update banner picture", http.StatusInternalServerError)
		return
	}

	// Save updated banner picture to the database
	if err := applyProfileUpdates(claims.Username, bannerUpdates); err != nil {
		http.Error(w, "Failed to update banner picture", http.StatusInternalServerError)
		return
	}

	// Respond with the updated profile
	if err := respondWithUserProfile(w, claims.Username); err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

func applyProfileUpdates(username string, updates ...bson.M) error {
	finalUpdate := bson.M{}
	for _, update := range updates {
		for key, value := range update {
			finalUpdate[key] = value
		}
	}

	_, err := userCollection.UpdateOne(
		context.TODO(),
		bson.M{"username": username},
		bson.M{"$set": finalUpdate},
	)
	return err
}
