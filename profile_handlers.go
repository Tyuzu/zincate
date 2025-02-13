package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"naevis/mq"
	"net/http"

	"github.com/golang-jwt/jwt/v5"
	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/crypto/bcrypt"
)

// Handlers for user profile

func getUserProfile(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	tokenString := r.Header.Get("Authorization")
	claims, err := validateJWT(tokenString)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	username := ps.ByName("username")

	// Retrieve user details
	var user User
	userCollection.FindOne(context.TODO(), bson.M{"username": username}).Decode(&user)

	// Retrieve follow data
	userFollow, err := GetUserFollowData(user.UserID)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Build and respond with the user profile
	userProfile := UserProfileResponse{
		UserID:         user.UserID,
		Username:       user.Username,
		Email:          user.Email,
		Name:           user.Name,
		Bio:            user.Bio,
		ProfilePicture: user.ProfilePicture,
		BannerPicture:  user.BannerPicture,
		Followerscount: len(userFollow.Followers),
		Followcount:    len(userFollow.Follows),
		IsFollowing:    contains(userFollow.Followers, claims.UserID),
	}

	fmt.Println("userFollow.Followers ::::: ", userFollow.Followers)
	fmt.Println("userFollow.Follows ::::: ", userFollow.Follows)
	fmt.Println("user.UserID ::::: ", user.UserID)
	fmt.Println("claims.UserID ::::: ", claims.UserID)

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

	mq.Emit("profile-edited")

	// Respond with the updated profile
	if err := respondWithUserProfile(w, claims.Username); err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

func getProfile(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	tokenString := r.Header.Get("Authorization")
	if tokenString == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

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

	// Retrieve follow data
	userFollow, err := GetUserFollowData(claims.UserID)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Retrieve user data
	var user User
	err = userCollection.FindOne(context.TODO(), bson.M{"userid": claims.UserID}).Decode(&user)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			http.Error(w, "User not found", http.StatusNotFound)
			return
		}
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Remove sensitive data
	user.Password = ""
	user.Followerscount = len(userFollow.Followers)
	user.Followcount = len(userFollow.Follows)

	// Convert user to JSON
	profileJSON, err := json.Marshal(user)
	if err != nil {
		http.Error(w, "Failed to encode profile", http.StatusInternalServerError)
		return
	}

	// Cache profile
	_ = CacheProfile(claims.Username, string(profileJSON))

	// Send response
	w.Header().Set("Content-Type", "application/json")
	w.Write(profileJSON)
}

func deleteProfile(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	tokenString := r.Header.Get("Authorization")
	claims, err := validateJWT(tokenString)
	if err != nil {
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
	mq.Emit("profile-deleted")

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
	if name := r.FormValue("name"); name != "" {
		update["name"] = name
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

	mq.Emit("profile-updated")

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

	mq.Emit("profilepic-updated")

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

	mq.Emit("bannerpic-updated")

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

func GetUserByUsername(username string) (*User, error) {
	var user User
	err := userCollection.FindOne(context.TODO(), bson.M{"username": username}).Decode(&user)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil // User not found
		}
		return nil, err
	}
	return &user, nil
}

func UpdateUserByUsername(username string, update bson.M) error {
	_, err := userCollection.UpdateOne(
		context.TODO(),
		bson.M{"username": username},
		bson.M{"$set": update},
	)
	return err
}

func DeleteUserByID(userID string) error {
	_, err := userCollection.DeleteOne(context.TODO(), bson.M{"userid": userID})
	return err
}

func GetUserFollowData(userID string) (UserFollow, error) {
	var userFollow UserFollow
	err := followingsCollection.FindOne(context.TODO(), bson.M{"userid": userID}).Decode(&userFollow)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return UserFollow{Followers: []string{}, Follows: []string{}}, nil // Return empty lists instead of nil
		}
		return userFollow, err
	}
	return userFollow, nil
}

func CacheProfile(username string, profileJSON string) error {
	return RdxSet("profile:"+username, profileJSON)
}

func GetCachedProfile(username string) (string, error) {
	return RdxGet("profile:" + username)
}

func InvalidateCachedProfile(username string) error {
	_, err := RdxDel("profile:" + username)
	return err
}
