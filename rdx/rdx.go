package rdx

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
)

//~ var redis_url = os.Getenv("REDIS_URL")
//~ var redis_pass = os.Getenv("REDIS_PASSWORD")

//~ var Conn = redis.NewClient(&redis.Options{
//~ Addr:     redis_url,
//~ Password: redis_pass, // no password set
//~ DB:       0,  // use default DB
//~ })

var rxdurl string = os.Getenv("REDIS_URL")
var rxdopts, _ = redis.ParseURL(rxdurl)
var Conn = redis.NewClient(rxdopts)

func init() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}
	RdxPing()
}

func RdxPing() {
	ctx := context.Background()
	k, err := Conn.Ping(ctx).Result()
	if err != nil {
		fmt.Printf("error while doing PING command in redis : %v", err)
	}
	fmt.Println(k)
}

func RdxSet(key, value string) error {

	ctx := context.Background()

	_, err := Conn.Set(ctx, key, value, 0).Result()
	if err != nil {
		return fmt.Errorf("error while doing SET command in redis : %v", err)
	}

	return err

}

func RdxGet(key string) (string, error) {

	ctx := context.Background()

	value, err := Conn.Get(ctx, key).Result()
	if err != nil {
		return "", fmt.Errorf("error while doing GET command in redis : %v", err)
	}

	return value, err
}

func RdxDel(key string) (string, error) {

	ctx := context.Background()

	value, err := Conn.Del(ctx, key).Result()
	if err != nil {
		return "", fmt.Errorf("error while doing DEL command in redis : %v", err)
	}

	return "" + string(rune(value)), err
}

func RdxHset(hash, key, value string) error {

	ctx := context.Background()

	_, err := Conn.HSet(ctx, hash, key, value).Result()
	if err != nil {
		return fmt.Errorf("error while doing HSET command in redis : %v", err)
	}

	return err
}

func RdxHget(hash, key string) (string, error) {

	ctx := context.Background()

	value, err := Conn.HGet(ctx, hash, key).Result()
	if err != nil {
		return "error : ", err
	}

	return value, err

}

func RdxHdel(hash, key string) (string, error) {

	ctx := context.Background()

	value, err := Conn.HDel(ctx, hash, key).Result()
	if err != nil {
		return string(rune(value)), fmt.Errorf("error while doing HGET command in redis : %v", err)
	}

	return string(rune(value)), err

}

func RdxHgetall(hash string) map[string]string {

	ctx := context.Background()
	value, _ := Conn.HGetAll(ctx, hash).Result()

	return value

}

func RdxAppend(key, value string) error {
	ctx := context.Background()
	_, err := Conn.Append(ctx, key, value).Result()
	if err != nil {
		return fmt.Errorf("error while doing APPEND command in redis : %v", err)
	}
	return err
}

func SetWithExpiry(key, value string, exptime time.Duration) error {
	ctx := context.Background()
	_, err := Conn.Set(ctx, key, value, exptime).Result()
	if err != nil {
		return fmt.Errorf("error while SetWithExpiry in redis : %v", err)
	}
	return err
}

func Exists(key string) bool {
	ctx := context.Background()
	exists, _ := Conn.Exists(ctx, key).Result()
	return exists > 0
}
