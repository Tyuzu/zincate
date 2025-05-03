package search

import (
	"fmt"
	"log"
	"naevis/rdx"
	"sort"
	"strings"
)

// -------------------------
// SEARCH FUNCTIONS
// -------------------------

// Search performs a basic search using the inverted index.
func Search(query string) ([]string, error) {
	query = strings.ToLower(query)
	// cacheKey := fmt.Sprintf("search:%s", query)
	// // Try Redis cache first.
	// if cached, err := GetCachedSearchResult(cacheKey); err == nil && len(cached) > 0 {
	// 	return cached, nil
	// }
	tokens := Tokenize(query)
	resultSet := make(map[string]bool)
	for _, token := range tokens {
		indexKey := fmt.Sprintf("inverted:%s", token)
		results, err := GetIndex(indexKey)
		if err != nil {
			return nil, err
		}
		for _, z := range results {
			resultSet[z.Member.(string)] = true
		}
	}
	var results []string
	for id := range resultSet {
		results = append(results, id)
	}
	// Optional: sort by recency based on the first token.
	if len(tokens) > 0 {
		sort.Slice(results, func(i, j int) bool {
			z1, err1 := rdx.Conn.ZScore(ctx, fmt.Sprintf("inverted:%s", tokens[0]), results[i]).Result()
			z2, err2 := rdx.Conn.ZScore(ctx, fmt.Sprintf("inverted:%s", tokens[0]), results[j]).Result()
			if err1 != nil || err2 != nil {
				return results[i] < results[j]
			}
			return z1 > z2
		})
	}
	// // Cache result.
	// if err := CacheSearchResult(cacheKey, results); err != nil {
	// 	log.Println("Error caching search result:", err)
	// }
	return results, nil
}

// SearchWithHashtagBoost boosts results that match hashtags.
func SearchWithHashtagBoost(query string) ([]string, error) {
	query = strings.ToLower(query)
	cacheKey := fmt.Sprintf("boost:%s", query)
	if cached, err := GetCachedSearchResult(cacheKey); err == nil && len(cached) > 0 {
		return cached, nil
	}
	tokens := Tokenize(query)
	hashtags := ExtractHashtags(query)
	freqMap := make(map[string]int)
	// Process normal tokens.
	for _, token := range tokens {
		indexKey := fmt.Sprintf("inverted:%s", token)
		results, err := GetIndex(indexKey)
		if err != nil {
			return nil, err
		}
		for _, z := range results {
			freqMap[z.Member.(string)] += 3
		}
	}
	// Process hashtags with extra boost.
	for _, tag := range hashtags {
		hashKey := fmt.Sprintf("inverted:%s", tag)
		results, err := GetIndex(hashKey)
		if err != nil {
			return nil, err
		}
		for _, z := range results {
			freqMap[z.Member.(string)] += 5
		}
	}
	var sortedResults []string
	for id := range freqMap {
		sortedResults = append(sortedResults, id)
	}
	sort.Slice(sortedResults, func(i, j int) bool {
		if freqMap[sortedResults[i]] != freqMap[sortedResults[j]] {
			return freqMap[sortedResults[i]] > freqMap[sortedResults[j]]
		}
		if len(tokens) > 0 {
			z1, err1 := rdx.Conn.ZScore(ctx, fmt.Sprintf("inverted:%s", tokens[0]), sortedResults[i]).Result()
			z2, err2 := rdx.Conn.ZScore(ctx, fmt.Sprintf("inverted:%s", tokens[0]), sortedResults[j]).Result()
			if err1 != nil || err2 != nil {
				return sortedResults[i] < sortedResults[j]
			}
			return z1 > z2
		}
		return sortedResults[i] < sortedResults[j]
	})
	if err := CacheSearchResult(cacheKey, sortedResults); err != nil {
		log.Println("Error caching hashtag boost search result:", err)
	}
	return sortedResults, nil
}

// GetResultsOfType retrieves entities by query and converts them to the result struct.
func GetIndexResults(entityType, query string) ([]string, error) {
	var ids []string
	var err error
	if strings.Contains(query, "#") {
		ids, err = SearchWithHashtagBoost(query)
	} else {
		ids, err = Search(query)
	}
	if err != nil {
		return nil, err
	}
	fmt.Println("ids", ids)
	return ids, nil
}
