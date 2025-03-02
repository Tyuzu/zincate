package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/joho/godotenv"
	"github.com/julienschmidt/httprouter"
	"github.com/rs/cors"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Security headers middleware
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Set HTTP headers for enhanced security
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		next.ServeHTTP(w, r) // Call the next handler
	})
}

type contextKey string

const userIDKey contextKey = "userId"

var (
	userCollection *mongo.Collection
	// profilesCollection *mongo.Collection
	userDataCollection   *mongo.Collection
	ticketsCollection    *mongo.Collection
	reviewsCollection    *mongo.Collection
	settingsCollection   *mongo.Collection
	followingsCollection *mongo.Collection
	placesCollection     *mongo.Collection
	businessesCollection *mongo.Collection
	bookingsCollection   *mongo.Collection
	menuCollection       *mongo.Collection
	promotionsCollection *mongo.Collection
	ownersCollection     *mongo.Collection
	postsCollection      *mongo.Collection
	seatsCollection      *mongo.Collection
	merchCollection      *mongo.Collection
	activitiesCollection *mongo.Collection
	eventsCollection     *mongo.Collection
	gigsCollection       *mongo.Collection
	mediaCollection      *mongo.Collection
	blogCollection       *mongo.Collection
	client               *mongo.Client
)

func main() {

	err := godotenv.Load()
	if err != nil {
		log.Fatalf("Error loading .env file")
	}

	// Get the MongoDB URI from the environment variable
	mongoURI := os.Getenv("MONGODB_URI")
	if mongoURI == "" {
		log.Fatalf("MONGODB_URI environment variable is not set")
	}

	// Use the SetServerAPIOptions() method to set the version of the Stable API on the client
	serverAPI := options.ServerAPI(options.ServerAPIVersion1)
	opts := options.Client().ApplyURI(mongoURI).SetServerAPIOptions(serverAPI)

	// Create a new client and connect to the server
	client, err := mongo.Connect(context.TODO(), opts)
	if err != nil {
		panic(err)
	}

	defer func() {
		if err = client.Disconnect(context.TODO()); err != nil {
			panic(err)
		}
	}()

	// Send a ping to confirm a successful connection
	if err := client.Database("admin").RunCommand(context.TODO(), bson.D{{Key: "ping", Value: 1}}).Err(); err != nil {
		panic(err)
	}
	fmt.Println("Pinged your deployment. You successfully connected to MongoDB!")

	settingsCollection = client.Database("eventdb").Collection("settings")
	reviewsCollection = client.Database("eventdb").Collection("reviews")
	followingsCollection = client.Database("eventdb").Collection("followings")
	// profilesCollection = client.Database("eventdb").Collection("users")
	userCollection = client.Database("eventdb").Collection("users")
	userDataCollection = client.Database("eventdb").Collection("userdata")
	ticketsCollection = client.Database("eventdb").Collection("ticks")
	placesCollection = client.Database("eventdb").Collection("places")
	businessesCollection = client.Database("eventdb").Collection("businesses")
	bookingsCollection = client.Database("eventdb").Collection("bookings")
	menuCollection = client.Database("eventdb").Collection("menu")
	promotionsCollection = client.Database("eventdb").Collection("promotions")
	ownersCollection = client.Database("eventdb").Collection("owners")
	postsCollection = client.Database("eventdb").Collection("posts")
	seatsCollection = client.Database("eventdb").Collection("seats")
	merchCollection = client.Database("eventdb").Collection("merch")
	activitiesCollection = client.Database("eventdb").Collection("activities")
	eventsCollection = client.Database("eventdb").Collection("events")
	gigsCollection = client.Database("eventdb").Collection("gigs")
	mediaCollection = client.Database("eventdb").Collection("media")
	blogCollection = client.Database("eventdb").Collection("blogs")

	router := httprouter.New()

	// Example Routes
	// router.GET("/", rateLimit(wrapHandler(proxyWithCircuitBreaker("frontend-service"))))

	router.POST("/api/activity/log", rateLimit(authenticate(logActivity)))
	router.GET("/api/activity/get", authenticate(getActivityFeed))

	router.POST("/api/auth/register", rateLimit(register))
	router.POST("/api/auth/login", rateLimit(login))
	router.POST("/api/auth/logout", authenticate(logoutUser))
	router.POST("/api/auth/token/refresh", rateLimit(authenticate(refreshToken)))

	// router.POST("/initialize", rateLimit(InitializeHandler))

	router.GET("/api/events/events", rateLimit(GetEvents))
	router.GET("/api/events/events/count", rateLimit(GetEventsCount))
	router.POST("/api/events/event", authenticate(CreateEvent))
	router.GET("/api/events/event/:eventid", GetEvent)
	router.PUT("/api/events/event/:eventid", authenticate(EditEvent))
	router.DELETE("/api/events/event/:eventid", authenticate(DeleteEvent))

	router.POST("/api/merch/event/:eventid", authenticate(createMerch))
	router.POST("/api/merch/event/:eventid/:merchid/buy", rateLimit(authenticate(buyMerch)))
	router.GET("/api/merch/event/:eventid", getMerchs)
	router.GET("/api/merch/event/:eventid/:merchid", getMerch)
	router.PUT("/api/merch/event/:eventid/:merchid", authenticate(editMerch))
	router.DELETE("/api/merch/event/:eventid/:merchid", authenticate(deleteMerch))

	router.POST("/api/merch/event/:eventid/:merchid/payment-session", authenticate(CreateMerchPaymentSession))
	router.POST("/api/merch/event/:eventid/:merchid/confirm-purchase", authenticate(ConfirmMerchPurchase))

	router.POST("/api/ticket/event/:eventid", authenticate(createTicket))
	router.GET("/api/ticket/event/:eventid", getTickets)
	router.GET("/api/ticket/event/:eventid/:ticketid", getTicket)
	router.PUT("/api/ticket/event/:eventid/:ticketid", authenticate(editTicket))
	router.DELETE("/api/ticket/event/:eventid/:ticketid", authenticate(deleteTicket))
	router.POST("/api/ticket/event/:eventid/:ticketid/buy", authenticate(buyTicket))

	router.POST("/api/ticket/event/:eventid/:ticketid/payment-session", authenticate(CreateTicketPaymentSession))
	router.GET("/api/events/event/:eventid/updates", EventUpdates)
	// router.POST("/api/seats/event/:eventid/:ticketid", rateLimit(authenticate(bookSeats)))
	// router.POST("/api/ticket/confirm-purchase", authenticate(ConfirmTicketPurchase))
	router.POST("/api/ticket/event/:eventid/:ticketid/confirm-purchase", authenticate(ConfirmTicketPurchase))

	router.GET("/api/seats/:eventid/available-seats", getAvailableSeats)
	router.POST("/api/seats/:eventid/lock-seats", lockSeats)
	router.POST("/api/seats/:eventid/unlock-seats", unlockSeats)
	router.POST("/api/seats/:eventid/ticket/:ticketid/confirm-purchase", confirmSeatPurchase)

	router.GET("/api/suggestions/places", rateLimit(suggestionsHandler))
	router.GET("/api/suggestions/follow", authenticate(suggestFollowers))

	router.GET("/api/search/:entityType", rateLimit(searchEvents))

	router.GET("/api/reviews/:entityType/:entityId", rateLimit(authenticate(getReviews)))
	router.GET("/api/reviews/:entityType/:entityId/:reviewId", authenticate(getReview))
	router.POST("/api/reviews/:entityType/:entityId", authenticate(addReview))
	router.PUT("/api/reviews/:entityType/:entityId/:reviewId", authenticate(editReview))
	router.DELETE("/api/reviews/:entityType/:entityId/:reviewId", authenticate(deleteReview))

	// Set up routes with middlewares
	router.POST("/api/media/:entitytype/:entityid", authenticate(addMedia))
	router.GET("/api/media/:entitytype/:entityid/:id", getMedia)
	router.PUT("/api/media/:entitytype/:entityid/:id", authenticate(editMedia))
	router.GET("/api/media/:entitytype/:entityid", rateLimit(getMedias))
	router.DELETE("/api/media/:entitytype/:entityid/:id", authenticate(deleteMedia))

	router.GET("/api/places/places", rateLimit(getPlaces))
	router.POST("/api/places/place", authenticate(createPlace))
	router.GET("/api/places/place/:placeid", getPlace)
	router.PUT("/api/places/place/:placeid", authenticate(editPlace))
	router.DELETE("/api/places/place/:placeid", authenticate(deletePlace))

	router.POST("/api/places/menu/:placeid", authenticate(createMenu))
	router.GET("/api/places/menu/:placeid", getMenus)
	router.GET("/api/places/menu/:placeid/:menuid", getMenu)
	router.PUT("/api/places/menu/:placeid/:menuid", authenticate(editMenu))
	router.DELETE("/api/places/menu/:placeid/:menuid", authenticate(deleteMenu))

	router.POST("/api/places/menu/:placeid/:menuid/payment-session", authenticate(CreateMenuPaymentSession))
	router.POST("/api/places/menu/:placeid/:menuid/confirm-purchase", authenticate(ConfirmMenuPurchase))

	router.GET("/api/profile/profile", authenticate(getProfile))
	router.PUT("/api/profile/edit", authenticate(editProfile))
	router.PUT("/api/profile/avatar", authenticate(editProfilePic))
	router.PUT("/api/profile/banner", authenticate(editProfileBanner))
	router.DELETE("/api/profile/delete", authenticate(deleteProfile))

	router.GET("/api/user/:username", rateLimit(getUserProfile))
	router.GET("/api/user/:username/data", rateLimit(authenticate(getUserProfileData)))

	router.PUT("/api/follows/:id", rateLimit(authenticate(ToggleFollow)))
	router.DELETE("/api/follows/:id", rateLimit(authenticate(ToggleUnFollow)))
	router.GET("/api/follows/:id/status", rateLimit(authenticate(DoesFollow)))
	router.GET("/api/followers/:id", rateLimit(authenticate(GetFollowers)))
	router.GET("/api/following/:id", rateLimit(authenticate(GetFollowing)))

	router.GET("/api/feed/feed", authenticate(GetPosts))
	router.GET("/api/feed/post/:postid", authenticate(GetPost))
	router.POST("/api/feed/post", rateLimit(authenticate(CreateTweetPost)))
	router.PUT("/api/feed/post/:postid", authenticate(EditPost))
	router.DELETE("/api/feed/post/:postid", authenticate(DeletePost))

	router.GET("/api/blog/blog", authenticate(GetBlogPosts))
	router.GET("/api/blog/post/:postid", authenticate(GetBlogPost))
	router.POST("/api/blog/post", rateLimit(authenticate(CreateBlogPost)))
	router.PUT("/api/blog/post/:postid", authenticate(EditBlogPost))
	router.DELETE("/api/blog/post/:postid", authenticate(DeleteBlogPost))

	router.GET("/api/settings/init/:userid", authenticate(initUserSettings))
	// router.GET("/api/settings/setting/:type", getUserSettings)
	router.GET("/api/settings/all", rateLimit(authenticate(getUserSettings)))
	router.PUT("/api/settings/setting/:type", rateLimit(authenticate(updateUserSetting)))

	router.GET("/api/sda/sda", rateLimit(authenticate(GetAds)))

	// CORS setup
	c := cors.New(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type", "Authorization"},
		AllowCredentials: true,
	})

	router.ServeFiles("/merchpic/*filepath", http.Dir("merchpic"))
	router.ServeFiles("/uploads/*filepath", http.Dir("uploads"))
	router.ServeFiles("/placepic/*filepath", http.Dir("placepic"))
	router.ServeFiles("/businesspic/*filepath", http.Dir("eventpic"))
	router.ServeFiles("/userpic/*filepath", http.Dir("userpic"))
	router.ServeFiles("/postpic/*filepath", http.Dir("postpic"))
	router.ServeFiles("/eventpic/*filepath", http.Dir("eventpic"))
	router.ServeFiles("/gigpic/*filepath", http.Dir("gigpic"))

	handler := securityHeaders(c.Handler(router))

	server := &http.Server{
		Addr:    ":4000",
		Handler: handler, // Use the middleware-wrapped handler
	}

	// Start server in a goroutine to handle graceful shutdown
	go func() {
		log.Println("Server started on port 4000")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Could not listen on port 4000: %v", err)
		}
	}()

	// Graceful shutdown listener
	shutdownChan := make(chan os.Signal, 1)
	signal.Notify(shutdownChan, os.Interrupt, syscall.SIGTERM)

	// Wait for termination signal
	<-shutdownChan
	log.Println("Shutting down gracefully...")

	// Attempt to gracefully shut down the server
	if err := server.Shutdown(context.Background()); err != nil {
		log.Fatalf("Server shutdown failed: %v", err)
	}
	log.Println("Server stopped")
}
