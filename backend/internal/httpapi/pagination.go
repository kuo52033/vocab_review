package httpapi

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

const maxPageLimit = 100

type pageQuery struct {
	Limit  int
	Offset int
}

func parsePageQuery(r *http.Request) (pageQuery, error) {
	values := r.URL.Query()
	limit, err := parseOptionalNonNegativeInt(values.Get("limit"), "limit")
	if err != nil {
		return pageQuery{}, err
	}
	offset, err := parseOptionalNonNegativeInt(values.Get("offset"), "offset")
	if err != nil {
		return pageQuery{}, err
	}
	if limit > maxPageLimit {
		return pageQuery{}, fmt.Errorf("limit must be %d or less", maxPageLimit)
	}
	return pageQuery{Limit: limit, Offset: offset}, nil
}

func parseOptionalNonNegativeInt(value string, name string) (int, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 0 {
		return 0, fmt.Errorf("%s must be a non-negative integer", name)
	}
	return parsed, nil
}
