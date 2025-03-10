package globals

import (
	"time"
)

const (
	RefreshTokenTTL = 7 * 24 * time.Hour // 7 days
	AccessTokenTTL  = 15 * time.Minute   // 15 minutes
)

var (
	// tokenSigningAlgo = jwt.SigningMethodHS256
	JwtSecret = []byte("your_secret_key") // Replace with a secure secret key
)

type ContextKey string

const UserIDKey ContextKey = "userId"
