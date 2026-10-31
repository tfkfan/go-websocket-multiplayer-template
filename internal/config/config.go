package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

const (
	WorldWidth   = 1200.0
	WorldHeight  = 800.0
	PlayerRadius = 25.0
	TickRate     = 30
)

type Config struct {
	Port              string
	WebSocketEndpoint string
	RoomEndpoint      string
	AssetsDir         string
	RoomTimeout       time.Duration
	RoomCapacity      int
}

func Read() (*Config, error) {
	if err := godotenv.Load(); err != nil {
		return nil, errors.New("failed to load .env")
	}

	var config Config

	port, err := lookupEnv("PORT")
	if err != nil {
		return nil, err
	}
	config.Port = port

	webSocketEndpoint, err := lookupEnv("WS_ENDPOINT")
	if err != nil {
		return nil, err
	}
	config.WebSocketEndpoint = webSocketEndpoint

	assetsDir, err := lookupEnv("ASSETS_DIR")
	if err != nil {
		return nil, err
	}
	config.AssetsDir = assetsDir

	roomEndpoint, err := lookupEnv("ROOM_ENDPOINT")
	if err != nil {
		return nil, err
	}
	config.RoomEndpoint = roomEndpoint

	roomTimeoutRaw, err := lookupEnv("ROOM_TIMEOUT")
	if err != nil {
		return nil, err
	}
	roomTimeout, err := time.ParseDuration(roomTimeoutRaw)
	if err != nil {
		return nil, err
	}
	if roomTimeout <= 0 {
		return nil, errors.New("ROOM_TIMEOUT must be positive")
	}
	config.RoomTimeout = roomTimeout

	roomCapacityRaw, err := lookupEnv("ROOM_CAPACITY")
	if err != nil {
		return nil, err
	}
	roomCapacity, err := strconv.Atoi(roomCapacityRaw)
	if err != nil {
		return nil, err
	}
	if roomCapacity <= 0 {
		return nil, errors.New("ROOM_CAPACITY must be positive")
	}
	config.RoomCapacity = roomCapacity

	return &config, nil
}

func lookupEnv(env string) (string, error) {
	value, exists := os.LookupEnv(env)
	if !exists {
		return "", fmt.Errorf("%s environment variable not set", env)
	}
	return value, nil
}
