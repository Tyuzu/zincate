package profile

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"naevis/db"
	"naevis/globals"
	"naevis/middleware"
	"naevis/mq"
	"naevis/rdx"
	"naevis/structs"
	"naevis/utils"
	"net/http"

	"github.com/golang-jwt/jwt/v5"
	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/crypto/bcrypt"
)

// Handlers for user profile
func GetUserProfile(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	tokenString := r.Header.Get("Authorization")
	if tokenString == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	// // Optional: Strip Bearer
	// if strings.HasPrefix(tokenString, "Bearer ") {
	// 	tokenString = strings.TrimPrefix(tokenString, "Bearer ")
	// }

	claims, err := ValidateJWT(tokenString)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	username := ps.ByName("username")

	var user structs.User
	err = db.UserCollection.FindOne(context.TODO(), bson.M{"username": username}).Decode(&user)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			http.Error(w, "User not found", http.StatusNotFound)
			return
		}
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	userFollow, err := GetUserFollowData(user.UserID)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	user.Online = rdx.Exists("online:" + user.UserID)

	userProfile := structs.UserProfileResponse{
		UserID:         user.UserID,
		Username:       user.Username,
		Email:          user.Email,
		Name:           user.Name,
		Bio:            user.Bio,
		ProfilePicture: user.ProfilePicture,
		BannerPicture:  user.BannerPicture,
		Followerscount: len(userFollow.Followers),
		Followcount:    len(userFollow.Follows),
		IsFollowing:    utils.Contains(userFollow.Followers, claims.UserID),
		Online:         user.Online,
		LastLogin:      user.LastLogin,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(userProfile)
}

// func GetUserProfile(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
// 	tokenString := r.Header.Get("Authorization")
// 	claims, err := ValidateJWT(tokenString)
// 	if err != nil {
// 		http.Error(w, "Unauthorized", http.StatusUnauthorized)
// 		return
// 	}

// 	username := ps.ByName("username")

// 	// Retrieve user details
// 	var user structs.User
// 	db.UserCollection.FindOne(context.TODO(), bson.M{"username": username}).Decode(&user)

// 	// Retrieve follow data
// 	userFollow, err := GetUserFollowData(user.UserID)
// 	if err != nil {
// 		http.Error(w, "Internal server error", http.StatusInternalServerError)
// 		return
// 	}

// 	// exists, _ := rdx.Conn.Exists(ctx, "online:"+uid).Result()
// 	user.Online = rdx.Exists("online:" + user.UserID)

// 	// isOnline, err := rdx.RdxGet("online:" + user.UserID)
// 	// if err != nil {
// 	// 	isOnline = "false"
// 	// }

// 	// Build and respond with the user profile
// 	userProfile := structs.UserProfileResponse{
// 		UserID:         user.UserID,
// 		Username:       user.Username,
// 		Email:          user.Email,
// 		Name:           user.Name,
// 		Bio:            user.Bio,
// 		ProfilePicture: user.ProfilePicture,
// 		BannerPicture:  user.BannerPicture,
// 		Followerscount: len(userFollow.Followers),
// 		Followcount:    len(userFollow.Follows),
// 		IsFollowing:    utils.Contains(userFollow.Followers, claims.UserID),
// 		Online:         user.Online,
// 	}

// 	fmt.Println("userFollow.Followers ::::: ", userFollow.Followers)
// 	fmt.Println("userFollow.Follows ::::: ", userFollow.Follows)
// 	fmt.Println("user.UserID ::::: ", user.UserID)
// 	fmt.Println("claims.UserID ::::: ", claims.UserID)

//		w.Header().Set("Content-Type", "application/json")
//		json.NewEncoder(w).Encode(userProfile)
//	}
func GetProfile(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	tokenString := r.Header.Get("Authorization")
	if tokenString == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	// if strings.HasPrefix(tokenString, "Bearer ") {
	// 	tokenString = strings.TrimPrefix(tokenString, "Bearer ")
	// }

	claims, err := ValidateJWT(tokenString)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	userFollow, err := GetUserFollowData(claims.UserID)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	var user structs.User
	err = db.UserCollection.FindOne(context.TODO(), bson.M{"userid": claims.UserID}).Decode(&user)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			http.Error(w, "User not found", http.StatusNotFound)
			return
		}
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	user.Password = "" // Clear password
	user.Followerscount = len(userFollow.Followers)
	user.Followcount = len(userFollow.Follows)
	user.Online = rdx.Exists("online:" + user.UserID)

	profileJSON, err := json.Marshal(user)
	if err != nil {
		http.Error(w, "Failed to encode profile", http.StatusInternalServerError)
		return
	}

	_ = CacheProfile(claims.Username, string(profileJSON))

	w.Header().Set("Content-Type", "application/json")
	w.Write(profileJSON)
}

// func GetProfile(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
// 	tokenString := r.Header.Get("Authorization")
// 	if tokenString == "" {
// 		http.Error(w, "Unauthorized", http.StatusUnauthorized)
// 		return
// 	}
// 	// tokenString := r.Header.Get("Authorization")
// 	// if tokenString == "" || len(tokenString) < 8 || tokenString[:7] != "Bearer " {
// 	// 	http.Error(w, "Unauthorized", http.StatusUnauthorized)
// 	// 	return
// 	// }
// 	// tokenString = tokenString[7:]

// 	claims, err := ValidateJWT(tokenString)
// 	if err != nil {
// 		http.Error(w, "Unauthorized", http.StatusUnauthorized)
// 		return
// 	}

// 	// // Check Redis cache
// 	// cachedProfile, err := GetCachedProfile(claims.Username)
// 	// if err == nil && cachedProfile != "" {
// 	// 	w.Header().Set("Content-Type", "application/json")
// 	// 	w.Write([]byte(cachedProfile))
// 	// 	return
// 	// }

// 	// Retrieve follow data
// 	userFollow, err := GetUserFollowData(claims.UserID)
// 	if err != nil {
// 		http.Error(w, "Internal server error", http.StatusInternalServerError)
// 		return
// 	}

// 	// Retrieve user data
// 	var user structs.User
// 	err = db.UserCollection.FindOne(context.TODO(), bson.M{"userid": claims.UserID}).Decode(&user)
// 	if err != nil {
// 		if errors.Is(err, mongo.ErrNoDocuments) {
// 			http.Error(w, "User not found", http.StatusNotFound)
// 			return
// 		}
// 		http.Error(w, "Internal server error", http.StatusInternalServerError)
// 		return
// 	}

// 	// Remove sensitive data
// 	user.Password = ""
// 	user.Followerscount = len(userFollow.Followers)
// 	user.Followcount = len(userFollow.Follows)

// 	user.Online = rdx.Exists("online:" + user.UserID)

// 	// --- in GetProfile, after you’ve loaded `user` ---
// 	// onlineKey := "online:" + claims.UserID
// 	// onlineVal, err := rdx.RdxGet(onlineKey)
// 	// if err != nil {
// 	// 	if err == redis.Nil {
// 	// 		// key doesn’t exist → offline
// 	// 		user.Online = "false"
// 	// 	} else {
// 	// 		log.Printf("redis GET %q error: %v", onlineKey, err)
// 	// 		user.Online = "false"
// 	// 	}
// 	// } else {
// 	// 	user.Online = onlineVal
// 	// }

// 	// Convert user to JSON
// 	profileJSON, err := json.Marshal(user)
// 	if err != nil {
// 		http.Error(w, "Failed to encode profile", http.StatusInternalServerError)
// 		return
// 	}

// 	// Cache profile
// 	_ = CacheProfile(claims.Username, string(profileJSON))

// 	// Send response
// 	w.Header().Set("Content-Type", "application/json")
// 	w.Write(profileJSON)
// }

func EditProfile(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	claims, err := ValidateJWT(r.Header.Get("Authorization"))
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
	updates, err := UpdateProfileFields(w, r, claims)
	if err != nil {
		http.Error(w, "Failed to update profile fields", http.StatusInternalServerError)
		return
	}

	// Save updates to the database
	if err := UpdateUserByUsername(claims.Username, updates); err != nil {
		http.Error(w, "Failed to update profile", http.StatusInternalServerError)
		return
	}

	m := mq.Index{EntityType: "profile", EntityId: claims.UserID, Method: "PUT"}
	go mq.Emit("profile-edited", m)

	// Respond with the updated profile
	if err := RespondWithUserProfile(w, claims.Username); err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

func DeleteProfile(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	tokenString := r.Header.Get("Authorization")
	claims, err := ValidateJWT(tokenString)
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

	m := mq.Index{EntityType: "profile", EntityId: claims.UserID, Method: "DELETE"}
	go mq.Emit("profile-deleted", m)

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Profile deleted successfully"})
}

func UpdateProfileFields(w http.ResponseWriter, r *http.Request, claims *middleware.Claims) (bson.M, error) {
	update := bson.M{}

	// Retrieve and update fields from the form
	if username := r.FormValue("username"); username != "" {
		update["username"] = username
		_ = rdx.RdxHset("users", claims.UserID, username)
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

	m := mq.Index{}
	mq.Notify("profile-updated", m)

	return update, nil
}

func ValidateJWT(tokenString string) (*middleware.Claims, error) {
	if tokenString == "" || len(tokenString) < 8 {
		return nil, fmt.Errorf("invalid token")
	}

	claims := &middleware.Claims{}
	_, err := jwt.ParseWithClaims(tokenString[7:], claims, func(token *jwt.Token) (any, error) {
		return globals.JwtSecret, nil
	})
	if err != nil {
		return nil, fmt.Errorf("unauthorized: %w", err)
	}
	return claims, nil
}

func RespondWithUserProfile(w http.ResponseWriter, username string) error {
	var userProfile structs.User
	err := db.UserCollection.FindOne(context.TODO(), bson.M{"username": username}).Decode(&userProfile)
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
func EditProfilePic(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	// Validate JWT token
	claims, err := ValidateJWT(r.Header.Get("Authorization"))
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
	if err := ApplyProfileUpdates(claims.Username, pictureUpdates); err != nil {
		http.Error(w, "Failed to update profile picture", http.StatusInternalServerError)
		return
	}

	m := mq.Index{}
	mq.Notify("profilepic-updated", m)

	// Respond with the updated profile
	if err := RespondWithUserProfile(w, claims.Username); err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// Update banner picture
func EditProfileBanner(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	// Validate JWT token
	claims, err := ValidateJWT(r.Header.Get("Authorization"))
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
	if err := ApplyProfileUpdates(claims.Username, bannerUpdates); err != nil {
		http.Error(w, "Failed to update banner picture", http.StatusInternalServerError)
		return
	}

	m := mq.Index{}
	mq.Notify("bannerpic-updated", m)

	// Respond with the updated profile
	if err := RespondWithUserProfile(w, claims.Username); err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

func ApplyProfileUpdates(username string, updates ...bson.M) error {
	finalUpdate := bson.M{}
	for _, update := range updates {
		for key, value := range update {
			finalUpdate[key] = value
		}
	}

	_, err := db.UserCollection.UpdateOne(
		context.TODO(),
		bson.M{"username": username},
		bson.M{"$set": finalUpdate},
	)
	return err
}

func GetUserByUsername(username string) (*structs.User, error) {
	var user structs.User
	err := db.UserCollection.FindOne(context.TODO(), bson.M{"username": username}).Decode(&user)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil // User not found
		}
		return nil, err
	}
	return &user, nil
}

func UpdateUserByUsername(username string, update bson.M) error {
	_, err := db.UserCollection.UpdateOne(
		context.TODO(),
		bson.M{"username": username},
		bson.M{"$set": update},
	)
	return err
}

func DeleteUserByID(userID string) error {
	_, err := db.UserCollection.DeleteOne(context.TODO(), bson.M{"userid": userID})
	return err
}

func GetUserFollowData(userID string) (structs.UserFollow, error) {
	var userFollow structs.UserFollow
	err := db.FollowingsCollection.FindOne(context.TODO(), bson.M{"userid": userID}).Decode(&userFollow)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return structs.UserFollow{Followers: []string{}, Follows: []string{}}, nil // Return empty lists instead of nil
		}
		return userFollow, err
	}
	return userFollow, nil
}

func CacheProfile(username string, profileJSON string) error {
	return rdx.RdxSet("profile:"+username, profileJSON)
}

func GetCachedProfile(username string) (string, error) {
	return rdx.RdxGet("profile:" + username)
}

func InvalidateCachedProfile(username string) error {
	_, err := rdx.RdxDel("profile:" + username)
	return err
}
