package main

import (
	"context"
	"fmt"
	"log"
	"naevis/activity"
	"naevis/ads"
	"naevis/auth"
	"naevis/db"
	"naevis/events"
	"naevis/feed"
	"naevis/media"
	"naevis/menu"
	"naevis/merch"
	"naevis/middleware"
	"naevis/places"
	"naevis/profile"
	"naevis/ratelim"
	"naevis/reviews"
	"naevis/search"
	"naevis/settings"
	"naevis/suggestions"
	"naevis/tickets"
	"naevis/userdata"
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

var (
	userCollection       *mongo.Collection
	userDataCollection   *mongo.Collection
	ticketsCollection    *mongo.Collection
	reviewsCollection    *mongo.Collection
	settingsCollection   *mongo.Collection
	followingsCollection *mongo.Collection
	placesCollection     *mongo.Collection
	menuCollection       *mongo.Collection
	postsCollection      *mongo.Collection
	merchCollection      *mongo.Collection
	activitiesCollection *mongo.Collection
	eventsCollection     *mongo.Collection
	mediaCollection      *mongo.Collection
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
	db.Client = client

	router := httprouter.New()

	// Example Routes
	// router.GET("/", rateLimit(wrapHandler(proxyWithCircuitBreaker("frontend-service"))))

	router.GET("/health", Index)
	router.GET("/api/search/:entityType", ratelim.RateLimit(search.SearchHandler))

	router.POST("/api/activity/log", ratelim.RateLimit(middleware.Authenticate(activity.LogActivity)))
	router.GET("/api/activity/get", middleware.Authenticate(activity.GetActivityFeed))

	router.POST("/api/auth/register", ratelim.RateLimit(auth.Register))
	router.POST("/api/auth/login", ratelim.RateLimit(auth.Login))
	router.POST("/api/auth/logout", middleware.Authenticate(auth.LogoutUser))
	router.POST("/api/auth/token/refresh", ratelim.RateLimit(middleware.Authenticate(auth.RefreshToken)))

	// router.POST("/initialize", ratelim.RateLimit(InitializeHandler))

	router.GET("/api/events/events", ratelim.RateLimit(events.GetEvents))
	router.GET("/api/events/events/count", ratelim.RateLimit(events.GetEventsCount))
	router.POST("/api/events/event", middleware.Authenticate(events.CreateEvent))
	router.GET("/api/events/event/:eventid", events.GetEvent)
	router.PUT("/api/events/event/:eventid", middleware.Authenticate(events.EditEvent))
	router.DELETE("/api/events/event/:eventid", middleware.Authenticate(events.DeleteEvent))
	router.POST("/api/events/event/:eventid/faqs", events.AddFAQs)

	router.POST("/api/merch/event/:eventid", middleware.Authenticate(merch.CreateMerch))
	router.POST("/api/merch/event/:eventid/:merchid/buy", ratelim.RateLimit(middleware.Authenticate(merch.BuyMerch)))
	router.GET("/api/merch/event/:eventid", merch.GetMerchs)
	router.GET("/api/merch/event/:eventid/:merchid", merch.GetMerch)
	router.PUT("/api/merch/event/:eventid/:merchid", middleware.Authenticate(merch.EditMerch))
	router.DELETE("/api/merch/event/:eventid/:merchid", middleware.Authenticate(merch.DeleteMerch))

	router.POST("/api/merch/event/:eventid/:merchid/payment-session", middleware.Authenticate(merch.CreateMerchPaymentSession))
	router.POST("/api/merch/event/:eventid/:merchid/confirm-purchase", middleware.Authenticate(merch.ConfirmMerchPurchase))

	router.POST("/api/ticket/event/:eventid", middleware.Authenticate(tickets.CreateTicket))
	router.GET("/api/ticket/event/:eventid", tickets.GetTickets)
	router.GET("/api/ticket/event/:eventid/:ticketid", tickets.GetTicket)
	router.PUT("/api/ticket/event/:eventid/:ticketid", middleware.Authenticate(tickets.EditTicket))
	router.DELETE("/api/ticket/event/:eventid/:ticketid", middleware.Authenticate(tickets.DeleteTicket))
	router.POST("/api/ticket/event/:eventid/:ticketid/buy", middleware.Authenticate(tickets.BuyTicket))

	// router.POST("/api/ticket/confirm-purchase", middleware.Authenticate(ConfirmTicketPurchase))
	router.POST("/api/ticket/event/:eventid/:ticketid/payment-session", middleware.Authenticate(tickets.CreateTicketPaymentSession))
	router.GET("/api/events/event/:eventid/updates", tickets.EventUpdates)
	// router.POST("/api/seats/event/:eventid/:ticketid", ratelim.RateLimit(middleware.Authenticate(bookSeats)))
	router.POST("/api/ticket/event/:eventid/:ticketid/confirm-purchase", middleware.Authenticate(tickets.ConfirmTicketPurchase))

	router.GET("/api/seats/:eventid/available-seats", tickets.GetAvailableSeats)
	router.POST("/api/seats/:eventid/lock-seats", tickets.LockSeats)
	router.POST("/api/seats/:eventid/unlock-seats", tickets.UnlockSeats)
	router.POST("/api/seats/:eventid/ticket/:ticketid/confirm-purchase", tickets.ConfirmSeatPurchase)

	router.GET("/api/suggestions/places/nearby", ratelim.RateLimit(suggestions.GetNearbyPlaces))
	router.GET("/api/suggestions/places", ratelim.RateLimit(suggestions.SuggestionsHandler))
	router.GET("/api/suggestions/follow", middleware.Authenticate(suggestions.SuggestFollowers))

	// router.GET("/api/search/:entityType", ratelim.RateLimit(searchEvents))

	router.GET("/api/reviews/:entityType/:entityId", ratelim.RateLimit(middleware.Authenticate(reviews.GetReviews)))
	router.GET("/api/reviews/:entityType/:entityId/:reviewId", middleware.Authenticate(reviews.GetReview))
	router.POST("/api/reviews/:entityType/:entityId", middleware.Authenticate(reviews.AddReview))
	router.PUT("/api/reviews/:entityType/:entityId/:reviewId", middleware.Authenticate(reviews.EditReview))
	router.DELETE("/api/reviews/:entityType/:entityId/:reviewId", middleware.Authenticate(reviews.DeleteReview))

	// Set up routes with middlewares
	router.POST("/api/media/:entitytype/:entityid", middleware.Authenticate(media.AddMedia))
	router.GET("/api/media/:entitytype/:entityid/:id", media.GetMedia)
	router.PUT("/api/media/:entitytype/:entityid/:id", middleware.Authenticate(media.EditMedia))
	router.GET("/api/media/:entitytype/:entityid", ratelim.RateLimit(media.GetMedias))
	router.DELETE("/api/media/:entitytype/:entityid/:id", middleware.Authenticate(media.DeleteMedia))

	router.GET("/api/places/places", ratelim.RateLimit(places.GetPlaces))
	router.POST("/api/places/place", middleware.Authenticate(places.CreatePlace))
	router.GET("/api/places/place/:placeid", places.GetPlace)
	router.PUT("/api/places/place/:placeid", middleware.Authenticate(places.EditPlace))
	router.DELETE("/api/places/place/:placeid", middleware.Authenticate(places.DeletePlace))

	router.POST("/api/places/menu/:placeid", middleware.Authenticate(menu.CreateMenu))
	router.GET("/api/places/menu/:placeid", menu.GetMenus)
	router.GET("/api/places/menu/:placeid/:menuid", menu.GetMenu)
	router.PUT("/api/places/menu/:placeid/:menuid", middleware.Authenticate(menu.EditMenu))
	router.DELETE("/api/places/menu/:placeid/:menuid", middleware.Authenticate(menu.DeleteMenu))

	router.POST("/api/places/menu/:placeid/:menuid/payment-session", middleware.Authenticate(menu.CreateMenuPaymentSession))
	router.POST("/api/places/menu/:placeid/:menuid/confirm-purchase", middleware.Authenticate(menu.ConfirmMenuPurchase))

	router.GET("/api/profile/profile", middleware.Authenticate(profile.GetProfile))
	router.PUT("/api/profile/edit", middleware.Authenticate(profile.EditProfile))
	router.PUT("/api/profile/avatar", middleware.Authenticate(profile.EditProfilePic))
	router.PUT("/api/profile/banner", middleware.Authenticate(profile.EditProfileBanner))
	router.DELETE("/api/profile/delete", middleware.Authenticate(profile.DeleteProfile))

	router.GET("/api/user/:username", ratelim.RateLimit(profile.GetUserProfile))
	router.GET("/api/user/:username/data", ratelim.RateLimit(middleware.Authenticate(userdata.GetUserProfileData)))

	router.PUT("/api/follows/:id", ratelim.RateLimit(middleware.Authenticate(profile.ToggleFollow)))
	router.DELETE("/api/follows/:id", ratelim.RateLimit(middleware.Authenticate(profile.ToggleUnFollow)))
	router.GET("/api/follows/:id/status", ratelim.RateLimit(middleware.Authenticate(profile.DoesFollow)))
	router.GET("/api/followers/:id", ratelim.RateLimit(middleware.Authenticate(profile.GetFollowers)))
	router.GET("/api/following/:id", ratelim.RateLimit(middleware.Authenticate(profile.GetFollowing)))

	router.GET("/api/feed/feed", middleware.Authenticate(feed.GetPosts))
	router.GET("/api/feed/post/:postid", feed.GetPost)
	router.POST("/api/feed/post", ratelim.RateLimit(middleware.Authenticate(feed.CreateTweetPost)))
	router.PUT("/api/feed/post/:postid", middleware.Authenticate(feed.EditPost))
	router.DELETE("/api/feed/post/:postid", middleware.Authenticate(feed.DeletePost))

	router.GET("/api/settings/init/:userid", middleware.Authenticate(settings.InitUserSettings))
	// router.GET("/api/settings/setting/:type", getUserSettings)
	router.GET("/api/settings/all", ratelim.RateLimit(middleware.Authenticate(settings.GetUserSettings)))
	router.PUT("/api/settings/setting/:type", ratelim.RateLimit(middleware.Authenticate(settings.UpdateUserSetting)))

	router.GET("/api/sda/sda", ratelim.RateLimit(middleware.Authenticate(ads.GetAds)))

	// CORS setup
	c := cors.New(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type", "Authorization"},
		AllowCredentials: true,
	})

	router.ServeFiles("/merchpic/*filepath", http.Dir("merchpic"))
	router.ServeFiles("/menupic/*filepath", http.Dir("menupic"))
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

func Index(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	fmt.Fprint(w, "200")
}
