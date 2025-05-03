package utils

import (
	_ "net/http/pprof"

	"github.com/google/uuid"
)

func GetUUID() string {
	return uuid.New().String()
}
