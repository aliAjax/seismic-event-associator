package application

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Address   string
	APIKey    string
	NodeID    string
	Timeout   time.Duration
	MaxBody   int64
	RateLimit int
	LogLevel  string
}

func DefaultConfig() Config {
	return Config{Address: ":18336", APIKey: "seismic-dev", NodeID: "seismic-node-1", Timeout: 10 * time.Second, MaxBody: 32 << 20, RateLimit: 100, LogLevel: "info"}
}
func Load(path string) (Config, error) {
	c := DefaultConfig()
	if path != "" {
		f, err := os.Open(path)
		if err != nil {
			return c, fmt.Errorf("open config: %w", err)
		}
		defer f.Close()
		scan := bufio.NewScanner(f)
		for scan.Scan() {
			line := strings.TrimSpace(scan.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			parts := strings.SplitN(line, ":", 2)
			if len(parts) != 2 {
				continue
			}
			if err := assign(&c, strings.TrimSpace(parts[0]), strings.Trim(strings.TrimSpace(parts[1]), "\"")); err != nil {
				return c, err
			}
		}
		if err := scan.Err(); err != nil {
			return c, err
		}
	}
	if v, ok := os.LookupEnv("SEISMIC_ADDRESS"); ok {
		c.Address = v
	}
	if v, ok := os.LookupEnv("SEISMIC_API_KEY"); ok {
		c.APIKey = v
	}
	if v, ok := os.LookupEnv("SEISMIC_NODE_ID"); ok {
		c.NodeID = v
	}
	if c.Address == "" || c.APIKey == "" || c.NodeID == "" || c.MaxBody < 1 {
		return c, fmt.Errorf("invalid configuration")
	}
	return c, nil
}
func assign(c *Config, k, v string) error {
	switch k {
	case "address":
		c.Address = v
	case "api_key":
		c.APIKey = v
	case "node_id":
		c.NodeID = v
	case "timeout":
		d, e := time.ParseDuration(v)
		if e != nil {
			return e
		}
		c.Timeout = d
	case "max_body":
		n, e := strconv.ParseInt(v, 10, 64)
		if e != nil {
			return e
		}
		c.MaxBody = n
	case "rate_limit":
		n, e := strconv.Atoi(v)
		if e != nil {
			return e
		}
		c.RateLimit = n
	case "log_level":
		c.LogLevel = v
	}
	return nil
}
