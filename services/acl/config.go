package main

import (
	"errors"
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type config struct {
	Server serverConfig `yaml:"server"`
	Redis  redisConfig  `yaml:"redis"`
}

type serverConfig struct {
	Addr        string `yaml:"addr"`
	EnableStats bool   `yaml:"enable_stats"`
}

type redisConfig struct {
	Addr         string `yaml:"addr"`
	Password     string `yaml:"password"`
	DB           int    `yaml:"db"`
	KeyPrefix    string `yaml:"key_prefix"`
	DialTimeout  string `yaml:"dial_timeout"`
	ReadTimeout  string `yaml:"read_timeout"`
	WriteTimeout string `yaml:"write_timeout"`
}

func defaultConfig() config {
	return config{
		Server: serverConfig{Addr: ":8888", EnableStats: false},
		Redis: redisConfig{
			Addr:         "",
			Password:     "",
			DB:           0,
			KeyPrefix:    "acl:",
			DialTimeout:  "1s",
			ReadTimeout:  "1s",
			WriteTimeout: "1s",
		},
	}
}

func loadConfig(path string) (config, bool, error) {
	cfg := defaultConfig()
	if path == "" {
		return cfg, false, nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, false, nil
		}
		return config{}, false, err
	}
	if err := yaml.Unmarshal(b, &cfg); err != nil {
		return config{}, false, fmt.Errorf("parse %s: %w", path, err)
	}
	if cfg.Server.Addr == "" {
		cfg.Server.Addr = defaultConfig().Server.Addr
	}
	if cfg.Redis.KeyPrefix == "" {
		cfg.Redis.KeyPrefix = defaultConfig().Redis.KeyPrefix
	}
	if cfg.Redis.DialTimeout == "" {
		cfg.Redis.DialTimeout = defaultConfig().Redis.DialTimeout
	}
	if cfg.Redis.ReadTimeout == "" {
		cfg.Redis.ReadTimeout = defaultConfig().Redis.ReadTimeout
	}
	if cfg.Redis.WriteTimeout == "" {
		cfg.Redis.WriteTimeout = defaultConfig().Redis.WriteTimeout
	}
	return cfg, true, nil
}

func (c config) redisTimeouts() (dial time.Duration, read time.Duration, write time.Duration, err error) {
	dial, err = time.ParseDuration(c.Redis.DialTimeout)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("redis.dial_timeout invalid duration: %w", err)
	}
	read, err = time.ParseDuration(c.Redis.ReadTimeout)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("redis.read_timeout invalid duration: %w", err)
	}
	write, err = time.ParseDuration(c.Redis.WriteTimeout)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("redis.write_timeout invalid duration: %w", err)
	}
	return dial, read, write, nil
}
