package main

// Profile caching (Redis)

func CacheProfile(username string, profileJSON string) error {
	return RdxSet("profile:"+username, profileJSON)
}

func GetCachedProfile(username string) (string, error) {
	return RdxGet("profile:" + username)
}

func InvalidateCachedProfile(username string) error {
	_, err := RdxDel("profile:" + username)
	return err
}
