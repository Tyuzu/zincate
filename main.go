package main

import (
	"context"
	"fmt"
	"log"
	"naevis/db"
	"naevis/ratelim"
	"naevis/routes"
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
		// w.Header().Set("Cache-Control", "max-age=0, no-cache, no-store, must-revalidate, private")
		next.ServeHTTP(w, r) // Call the next handler
	})
}

var (
	userCollection             *mongo.Collection
	iternaryCollection         *mongo.Collection
	userDataCollection         *mongo.Collection
	ticketsCollection          *mongo.Collection
	reviewsCollection          *mongo.Collection
	settingsCollection         *mongo.Collection
	followingsCollection       *mongo.Collection
	placesCollection           *mongo.Collection
	menuCollection             *mongo.Collection
	postsCollection            *mongo.Collection
	merchCollection            *mongo.Collection
	activitiesCollection       *mongo.Collection
	eventsCollection           *mongo.Collection
	mediaCollection            *mongo.Collection
	filesCollection            *mongo.Collection
	artistsCollection          *mongo.Collection
	cartoonsCollection         *mongo.Collection
	purchasedTicketsCollection *mongo.Collection
	bookingsCollection         *mongo.Collection
	slotCollection             *mongo.Collection
	artistEventsCollection     *mongo.Collection
	artistSongsCollection      *mongo.Collection
)

// Set up all routes and middleware layers
func setupRouter(rateLimiter *ratelim.RateLimiter) http.Handler {
	router := httprouter.New()
	router.GET("/health", Index)

	routes.AddActivityRoutes(router)
	routes.AddAuthRoutes(router)
	routes.AddEventsRoutes(router)
	routes.AddMerchRoutes(router)
	routes.AddTicketRoutes(router)
	routes.AddSuggestionsRoutes(router)
	routes.AddReviewsRoutes(router)
	routes.AddMediaRoutes(router)
	routes.AddPlaceRoutes(router)
	routes.AddProfileRoutes(router)
	routes.AddArtistRoutes(router)
	routes.AddCartoonRoutes(router)
	routes.AddMapRoutes(router)
	routes.AddItineraryRoutes(router)
	routes.AddFeedRoutes(router, rateLimiter)
	routes.AddSettingsRoutes(router)
	routes.AddAdsRoutes(router)
	routes.AddHomeFeedRoutes(router)
	routes.AddSearchRoutes(router)
	routes.AddStaticRoutes(router)

	// CORS setup (adjust AllowedOrigins in production)
	c := cors.New(cors.Options{
		AllowedOrigins:   []string{"*"}, // Consider specific origins in production
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type", "Authorization"},
		AllowCredentials: true,
	})

	// Wrap handlers with middleware: CORS -> Security -> Logging -> Router
	return loggingMiddleware(securityHeaders(c.Handler(router)))
}

// Middleware: Simple request logging
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[%s] %s %s", r.Method, r.RequestURI, r.RemoteAddr)
		next.ServeHTTP(w, r)
	})
}

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

	iternaryCollection = client.Database("eventdb").Collection("itinerary")
	db.ItineraryCollection = iternaryCollection
	settingsCollection = client.Database("eventdb").Collection("settings")
	db.SettingsCollection = settingsCollection
	reviewsCollection = client.Database("eventdb").Collection("reviews")
	db.ReviewsCollection = reviewsCollection
	followingsCollection = client.Database("eventdb").Collection("followings")
	db.FollowingsCollection = followingsCollection
	userCollection = client.Database("eventdb").Collection("users")
	db.UserCollection = userCollection
	userDataCollection = client.Database("eventdb").Collection("userdata")
	db.UserDataCollection = userDataCollection
	ticketsCollection = client.Database("eventdb").Collection("ticks")
	db.TicketsCollection = ticketsCollection
	placesCollection = client.Database("eventdb").Collection("places")
	db.PlacesCollection = placesCollection
	postsCollection = client.Database("eventdb").Collection("posts")
	db.PostsCollection = postsCollection
	merchCollection = client.Database("eventdb").Collection("merch")
	db.MerchCollection = merchCollection
	menuCollection = client.Database("eventdb").Collection("menu")
	db.MenuCollection = menuCollection
	activitiesCollection = client.Database("eventdb").Collection("activities")
	db.ActivitiesCollection = activitiesCollection
	eventsCollection = client.Database("eventdb").Collection("events")
	db.EventsCollection = eventsCollection
	mediaCollection = client.Database("eventdb").Collection("media")
	db.MediaCollection = mediaCollection
	filesCollection = client.Database("eventdb").Collection("files")
	db.FilesCollection = filesCollection
	artistsCollection = client.Database("eventdb").Collection("artists")
	db.ArtistsCollection = artistsCollection
	purchasedTicketsCollection = client.Database("eventdb").Collection("purticks")
	db.PurchasedTicketsCollection = purchasedTicketsCollection
	bookingsCollection = client.Database("eventdb").Collection("bookings")
	db.BookingsCollection = bookingsCollection
	slotCollection = client.Database("eventdb").Collection("slots")
	db.SlotCollection = slotCollection
	artistEventsCollection = client.Database("eventdb").Collection("artistevents")
	db.ArtistEventsCollection = artistEventsCollection
	artistSongsCollection = client.Database("eventdb").Collection("songs")
	db.ArtistSongsCollection = artistSongsCollection
	// GigsCollection = Client.Database("eventdb").Collection("gigs")
	cartoonsCollection = client.Database("eventdb").Collection("cartoons")
	db.CartoonsCollection = cartoonsCollection
	db.Client = client

	router := httprouter.New()

	rateLimiter := ratelim.NewRateLimiter()
	handler := setupRouter(rateLimiter)

	router.GET("/health", Index)

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

func Index(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	fmt.Fprint(w, "200")
}
