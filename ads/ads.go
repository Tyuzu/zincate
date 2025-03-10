package ads

import (
	"encoding/json"
	"net/http"

	"github.com/julienschmidt/httprouter"
)

// Ad represents the structure of an advertisement.
type Ad struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Image       string `json:"image"`
	Link        string `json:"link"`
	Category    string `json:"category"`
}

// Dummy ad data
var ads = []Ad{
	{
		ID:          "1",
		Title:       "Tech Gadget Sale",
		Description: "Get the latest gadgets at unbeatable prices!",
		Image:       "https://via.placeholder.com/300x150?text=Tech+Ad",
		Link:        "https://example.com/tech-sale",
		Category:    "tech",
	},
	{
		ID:          "2",
		Title:       "Travel Deals",
		Description: "Explore the world with our exclusive travel packages.",
		Image:       "https://via.placeholder.com/300x150?text=Travel+Ad",
		Link:        "https://example.com/travel-deals",
		Category:    "travel",
	},
}

// GetAds handles the API request to fetch ads.
func GetAds(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	category := r.URL.Query().Get("category")
	filteredAds := []Ad{}

	for _, ad := range ads {
		if ad.Category == category {
			filteredAds = append(filteredAds, ad)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(filteredAds)
}
