package routes

import (
	"naevis/activity"
	"naevis/ads"
	"naevis/agi"
	"naevis/artists"
	"naevis/auth"
	"naevis/booking"
	"naevis/cartoons"
	"naevis/chathandlers"
	"naevis/comments"
	"naevis/events"
	"naevis/feed"
	"naevis/itinerary"
	"naevis/maps"
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
	"naevis/websock"
	"net/http"
	_ "net/http/pprof"

	"github.com/julienschmidt/httprouter"
)

func AddStaticRoutes(router *httprouter.Router) {
	router.ServeFiles("/static/postpic/*filepath", http.Dir("static/postpic"))
	router.ServeFiles("/static/merchpic/*filepath", http.Dir("static/merchpic"))
	router.ServeFiles("/static/menupic/*filepath", http.Dir("static/menupic"))
	router.ServeFiles("/static/uploads/*filepath", http.Dir("static/uploads"))
	router.ServeFiles("/static/placepic/*filepath", http.Dir("static/placepic"))
	router.ServeFiles("/static/businesspic/*filepath", http.Dir("static/eventpic"))
	router.ServeFiles("/static/userpic/*filepath", http.Dir("static/userpic"))
	router.ServeFiles("/static/eventpic/*filepath", http.Dir("static/eventpic"))
	router.ServeFiles("/static/artistpic/*filepath", http.Dir("static/artistpic"))
	router.ServeFiles("/static/cartoonpic/*filepath", http.Dir("static/cartoonpic"))
	router.ServeFiles("/static/chatpic/*filepath", http.Dir("static/chatpic"))
}

func AddActivityRoutes(router *httprouter.Router) {
	router.POST("/api/activity/log", ratelim.RateLimit(middleware.Authenticate(activity.LogActivities)))
	router.GET("/api/activity/get", middleware.Authenticate(activity.GetActivityFeed))

}

func AddCommentsRoutes(router *httprouter.Router) {
	router.POST("/api/comments/:entitytype/:entityid", comments.CreateComment)
	router.GET("/api/comments/:entitytype/:entityid", comments.GetComments)
	router.PUT("/api/comments/:entitytype/:entityid/:commentid", comments.UpdateComment)
	router.DELETE("/api/comments/:entitytype/:entityid/:commentid", comments.DeleteComment)
}

func AddAuthRoutes(router *httprouter.Router) {
	router.POST("/api/auth/register", ratelim.RateLimit(auth.Register))
	router.POST("/api/auth/login", ratelim.RateLimit(auth.Login))
	router.POST("/api/auth/logout", middleware.Authenticate(auth.LogoutUser))
	router.POST("/api/auth/token/refresh", ratelim.RateLimit(middleware.Authenticate(auth.RefreshToken)))

	router.POST("/api/auth/verify-otp", ratelim.RateLimit(auth.VerifyOTPHandler))
	router.POST("/api/auth/request-otp", ratelim.RateLimit(auth.VerifyOTPHandler))
}

func AddBookingRoutes(router *httprouter.Router) {
	router.POST("/api/slots", ratelim.RateLimit(booking.AddSlot))
	router.DELETE("/api/slots/:date/:time", ratelim.RateLimit(booking.DeleteSlot))
	router.GET("/api/slots/:date", middleware.Authenticate(booking.GetSlotsByDate))
	router.GET("/api/bookings/:date", ratelim.RateLimit(middleware.Authenticate(booking.GetBookingsByDate)))
	router.POST("/api/bookings", ratelim.RateLimit(middleware.Authenticate(booking.CreateBooking)))
}

func AddEventsRoutes(router *httprouter.Router) {
	router.GET("/api/events/events", ratelim.RateLimit(events.GetEvents))
	router.GET("/api/events/events/count", ratelim.RateLimit(events.GetEventsCount))
	router.POST("/api/events/event", middleware.Authenticate(events.CreateEvent))
	router.GET("/api/events/event/:eventid", events.GetEvent)
	router.PUT("/api/events/event/:eventid", middleware.Authenticate(events.EditEvent))
	router.DELETE("/api/events/event/:eventid", middleware.Authenticate(events.DeleteEvent))
	router.POST("/api/events/event/:eventid/faqs", events.AddFAQs)
}

func AddMerchRoutes(router *httprouter.Router) {
	router.POST("/api/merch/:entityType/:eventid", middleware.Authenticate(merch.CreateMerch))
	router.POST("/api/merch/:entityType/:eventid/:merchid/buy", ratelim.RateLimit(middleware.Authenticate(merch.BuyMerch)))
	router.GET("/api/merch/:entityType/:eventid", merch.GetMerchs)
	router.GET("/api/merch/:entityType/:eventid/:merchid", merch.GetMerch)
	router.PUT("/api/merch/:entityType/:eventid/:merchid", middleware.Authenticate(merch.EditMerch))
	router.DELETE("/api/merch/:entityType/:eventid/:merchid", middleware.Authenticate(merch.DeleteMerch))

	router.POST("/api/merch/:entityType/:eventid/:merchid/payment-session", middleware.Authenticate(merch.CreateMerchPaymentSession))
	router.POST("/api/merch/:entityType/:eventid/:merchid/confirm-purchase", middleware.Authenticate(merch.ConfirmMerchPurchase))

}

func AddTicketRoutes(router *httprouter.Router) {
	router.POST("/api/ticket/event/:eventid", ratelim.RateLimit(middleware.Authenticate(tickets.CreateTicket)))
	router.GET("/api/ticket/event/:eventid", ratelim.RateLimit(tickets.GetTickets))
	router.GET("/api/ticket/event/:eventid/:ticketid", ratelim.RateLimit(tickets.GetTicket))
	router.PUT("/api/ticket/event/:eventid/:ticketid", ratelim.RateLimit(middleware.Authenticate(tickets.EditTicket)))
	router.DELETE("/api/ticket/event/:eventid/:ticketid", ratelim.RateLimit(middleware.Authenticate(tickets.DeleteTicket)))
	router.POST("/api/ticket/event/:eventid/:ticketid/buy", ratelim.RateLimit(middleware.Authenticate(tickets.BuyTicket)))
	router.GET("/api/ticket/verify/:eventid", ratelim.RateLimit(tickets.VerifyTicket))
	router.GET("/api/ticket/print/:eventid", ratelim.RateLimit(tickets.PrintTicket))

	// router.POST("/api/ticket/confirm-purchase", middleware.Authenticate(ConfirmTicketPurchase))
	router.POST("/api/ticket/event/:eventid/:ticketid/payment-session", ratelim.RateLimit(middleware.Authenticate(tickets.CreateTicketPaymentSession)))
	router.GET("/api/events/event/:eventid/updates", ratelim.RateLimit(tickets.EventUpdates))
	// router.POST("/api/seats/event/:eventid/:ticketid", ratelim.RateLimit(middleware.Authenticate(bookSeats)))
	router.POST("/api/ticket/event/:eventid/:ticketid/confirm-purchase", ratelim.RateLimit(middleware.Authenticate(tickets.ConfirmTicketPurchase)))

	router.GET("/api/seats/:eventid/available-seats", ratelim.RateLimit(tickets.GetAvailableSeats))
	router.POST("/api/seats/:eventid/lock-seats", ratelim.RateLimit(tickets.LockSeats))
	router.POST("/api/seats/:eventid/unlock-seats", ratelim.RateLimit(tickets.UnlockSeats))
	router.POST("/api/seats/:eventid/ticket/:ticketid/confirm-purchase", ratelim.RateLimit(tickets.ConfirmSeatPurchase))
	router.GET("/api/ticket/event/:eventid/:ticketid/seats", ratelim.RateLimit(tickets.GetTicketSeats))
}

func AddSuggestionsRoutes(router *httprouter.Router) {
	router.GET("/api/suggestions/places/nearby", ratelim.RateLimit(suggestions.GetNearbyPlaces))
	router.GET("/api/suggestions/places", ratelim.RateLimit(suggestions.SuggestionsHandler))
	router.GET("/api/suggestions/follow", ratelim.RateLimit(middleware.Authenticate(suggestions.SuggestFollowers)))
}

func AddReviewsRoutes(router *httprouter.Router) {
	router.GET("/api/reviews/:entityType/:entityId", ratelim.RateLimit(middleware.Authenticate(reviews.GetReviews)))
	router.GET("/api/reviews/:entityType/:entityId/:reviewId", ratelim.RateLimit(middleware.Authenticate(reviews.GetReview)))
	router.POST("/api/reviews/:entityType/:entityId", ratelim.RateLimit(middleware.Authenticate(reviews.AddReview)))
	router.PUT("/api/reviews/:entityType/:entityId/:reviewId", ratelim.RateLimit(middleware.Authenticate(reviews.EditReview)))
	router.DELETE("/api/reviews/:entityType/:entityId/:reviewId", ratelim.RateLimit(middleware.Authenticate(reviews.DeleteReview)))
}

func AddMediaRoutes(router *httprouter.Router) {
	// Set up routes with middlewares
	router.POST("/api/media/:entitytype/:entityid", ratelim.RateLimit(middleware.Authenticate(media.AddMedia)))
	router.GET("/api/media/:entitytype/:entityid/:id", ratelim.RateLimit(media.GetMedia))
	router.PUT("/api/media/:entitytype/:entityid/:id", ratelim.RateLimit(middleware.Authenticate(media.EditMedia)))
	router.GET("/api/media/:entitytype/:entityid", ratelim.RateLimit(media.GetMedias))
	router.DELETE("/api/media/:entitytype/:entityid/:id", ratelim.RateLimit(middleware.Authenticate(media.DeleteMedia)))
}

func AddPlaceRoutes(router *httprouter.Router) {
	router.GET("/api/places/places", ratelim.RateLimit(places.GetPlaces))
	router.POST("/api/places/place", middleware.Authenticate(places.CreatePlace))
	router.GET("/api/places/place/:placeid", places.GetPlace)
	router.GET("/api/places/place-details", places.GetPlaceQ)
	router.PUT("/api/places/place/:placeid", middleware.Authenticate(places.EditPlace))
	router.DELETE("/api/places/place/:placeid", middleware.Authenticate(places.DeletePlace))

	router.POST("/api/places/menu/:placeid", middleware.Authenticate(menu.CreateMenu))
	router.GET("/api/places/menu/:placeid", menu.GetMenus)
	router.GET("/api/places/menu/:placeid/:menuid", menu.GetMenu)
	router.PUT("/api/places/menu/:placeid/:menuid", middleware.Authenticate(menu.EditMenu))
	router.DELETE("/api/places/menu/:placeid/:menuid", middleware.Authenticate(menu.DeleteMenu))

	router.POST("/api/places/menu/:placeid/:menuid/payment-session", middleware.Authenticate(menu.CreateMenuPaymentSession))
	router.POST("/api/places/menu/:placeid/:menuid/confirm-purchase", middleware.Authenticate(menu.ConfirmMenuPurchase))

}

func AddProfileRoutes(router *httprouter.Router) {
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

}

func AddArtistRoutes(router *httprouter.Router) {
	router.GET("/api/artists", artists.GetAllArtists)
	router.GET("/api/artists/:id", artists.GetArtistByID)
	router.DELETE("/api/artists/:id", artists.DeleteArtistByID)
	router.GET("/api/events/event/:eventid/artists", artists.GetArtistsByEvent)
	router.POST("/api/artists", artists.CreateArtist)
	router.PUT("/api/artists/:id", artists.UpdateArtist)

	router.GET("/api/artists/:id/songs", artists.GetArtistsSongs)
	router.POST("/api/artists/:id/songs", artists.PostNewSong)
	router.DELETE("/api/artists/:id/songs/:songId", artists.DeleteSong)
	router.POST("/api/artists/:id/songs/:songId/edit", artists.EditSong) // ← new route

	router.GET("/api/artists/:id/albums", artists.GetArtistsAlbums)
	router.GET("/api/artists/:id/posts", artists.GetArtistsPosts)
	router.GET("/api/artists/:id/merch", artists.GetArtistsMerch)

	router.GET("/api/artists/:id/events", artists.GetArtistEvents)
	router.POST("/api/artists/:id/events", artists.CreateArtistEvent)
	router.PUT("/api/artists/:id/events", artists.UpdateArtistEvent)
	router.DELETE("/api/artists/:id/events", artists.DeleteArtistEvent)
}

func AddCartoonRoutes(router *httprouter.Router) {
	router.GET("/api/cartoons", cartoons.GetAllCartoons)
	router.GET("/api/cartoons/:id", cartoons.GetCartoonByID)
	router.GET("/api/events/event/:eventid/cartoons", cartoons.GetCartoonsByEvent)
	router.POST("/api/cartoons", cartoons.CreateCartoon)
	router.PUT("/api/cartoons/:id", cartoons.UpdateCartoon)

}

func AddChatRoutes(router *httprouter.Router) {

	// Existing endpoints.
	router.GET("/api/chat/contacts", middleware.Authenticate(chathandlers.ContactsHandler))
	router.GET("/api/chat/chats", middleware.Authenticate(chathandlers.ChatsHandler))
	router.GET("/api/chat/messages", middleware.Authenticate(chathandlers.MessagesHandler))
	router.POST("/api/chat/messages/send", middleware.Authenticate(chathandlers.SendMessageHandler))
	router.PUT("/api/chat/messages/edit", middleware.Authenticate(chathandlers.EditMessageHandler))
	router.DELETE("/api/chat/messages/delete", middleware.Authenticate(chathandlers.DeleteMessageHandler))
	router.DELETE("/api/chat/chats/:chatid", middleware.Authenticate(chathandlers.DeleteChatHandler))
	router.GET("/ws", websock.WsHandler)
	router.POST("/api/chat/chats/create", chathandlers.CreateChatHandler)
}

func AddMapRoutes(router *httprouter.Router) {
	router.GET("/api/maps/config/:entity", maps.GetMapConfig)
	router.GET("/api/maps/markers/:entity", maps.GetMapMarkers)

}

func AddItineraryRoutes(router *httprouter.Router) {
	router.GET("/api/itineraries", itinerary.GetItineraries)               //Fetch all itineraries
	router.POST("/api/itineraries", itinerary.CreateItinerary)             //Create a new itinerary
	router.GET("/api/itineraries/all/:id", itinerary.GetItinerary)         //Fetch a single itinerary
	router.PUT("/api/itineraries/:id", itinerary.UpdateItinerary)          //Update an itinerary
	router.DELETE("/api/itineraries/:id", itinerary.DeleteItinerary)       //Delete an itinerary
	router.GET("/api/itineraries/search", itinerary.SearchItineraries)     //Search an itinerary
	router.POST("/api/itineraries/:id/fork", itinerary.ForkItinerary)      //Fork a new itinerary
	router.PUT("/api/itineraries/:id/publish", itinerary.PublishItinerary) //Publish an itinerary
}

func AddFeedRoutes(router *httprouter.Router, rateLimiter *ratelim.RateLimiter) {
	router.GET("/api/feed/feed", middleware.Authenticate(feed.GetPosts))
	router.GET("/api/feed/post/:postid", rateLimiter.Limit(feed.GetPost))
	router.POST("/api/check-file", rateLimiter.Limit(middleware.Authenticate(feed.CheckUserInFile)))
	// router.POST("/api/feed/repost/:postid", feed.Repost)
	// router.DELETE("/api/feed/repost/:postid", feed.DeleteRepost)
	router.POST("/api/feed/post", ratelim.RateLimit(middleware.Authenticate(feed.CreateTweetPost)))
	router.PUT("/api/feed/post/:postid", middleware.Authenticate(feed.EditPost))
	router.DELETE("/api/feed/post/:postid", middleware.Authenticate(feed.DeletePost))
}

func AddSettingsRoutes(router *httprouter.Router) {
	router.GET("/api/settings/init/:userid", middleware.Authenticate(settings.InitUserSettings))
	// router.GET("/api/settings/setting/:type", getUserSettings)
	router.GET("/api/settings/all", ratelim.RateLimit(middleware.Authenticate(settings.GetUserSettings)))
	router.PUT("/api/settings/setting/:type", ratelim.RateLimit(middleware.Authenticate(settings.UpdateUserSetting)))
}

func AddAdsRoutes(router *httprouter.Router) {
	router.GET("/api/sda/sda", ratelim.RateLimit(middleware.Authenticate(ads.GetAds)))
}

func AddHomeFeedRoutes(router *httprouter.Router) {
	router.POST("/agi/home_feed_section", ratelim.RateLimit(agi.GetHomeFeed))

}
func AddSearchRoutes(router *httprouter.Router) {
	router.GET("/api/ac", search.Autocompleter)
	router.GET("/api/search/:entityType", ratelim.RateLimit(search.SearchHandler))
	router.POST("/emitted", search.EventHandler)
}

func AddMiscRoutes(router *httprouter.Router) {
	// Example Routes
	// router.GET("/", ratelim.RateLimit(wrapHandler(proxyWithCircuitBreaker("frontend-service"))))

	// router.GET("/api/search/:entityType", ratelim.RateLimit(searchEvents))

	// router.POST("/api/check-file", rateLimiter.Limit(filecheck.CheckFileExists))
	// router.POST("/api/upload", rateLimiter.Limit(filecheck.UploadFile))
	// router.POST("/api/feed/remhash", rateLimiter.Limit(filecheck.RemoveUserFile))

	// router.POST("/agi/home_feed_section", ratelim.RateLimit(middleware.Authenticate(agi.GetHomeFeed)))
	// router.GET("/resize/:folder/*filename", cdn.ServeStatic)

}
