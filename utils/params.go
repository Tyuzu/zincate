package utils

import (
	"net/http"
	_ "net/http/pprof"
	"strconv"
	"strings"
)

type QueryOptions struct {
	Page      int
	Limit     int
	Published *bool
	Search    string
	Genre     string
}

func ParseQueryOptions(r *http.Request) QueryOptions {
	q := r.URL.Query()

	page, _ := strconv.Atoi(q.Get("page"))
	if page < 1 {
		page = 1
	}

	limit, _ := strconv.Atoi(q.Get("limit"))
	if limit < 1 {
		limit = 10
	}

	var published *bool
	if pubStr := q.Get("published"); pubStr != "" {
		val := pubStr == "true"
		published = &val
	}

	return QueryOptions{
		Page:      page,
		Limit:     limit,
		Published: published,
		Search:    q.Get("search"),
		Genre:     q.Get("genre"),
	}
}

func ContainsIgnoreCase(str, substr string) bool {
	return strings.Contains(strings.ToLower(str), strings.ToLower(substr))
}
