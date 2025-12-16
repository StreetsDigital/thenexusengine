// Package redis provides a Redis client for PBS
package redis

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"time"

	"github.com/rs/zerolog/log"
)

// Client wraps a Redis connection
type Client struct {
	conn    Conn
	address string
}

// Conn is the interface for Redis connection operations
type Conn interface {
	Do(ctx context.Context, args ...interface{}) (interface{}, error)
	Close() error
}

// SimpleConn is a minimal Redis connection using net
type SimpleConn struct {
	addr     string
	password string
	db       int
}

// New creates a new Redis client from a URL
func New(redisURL string) (*Client, error) {
	if redisURL == "" {
		return nil, fmt.Errorf("redis URL is empty")
	}

	u, err := url.Parse(redisURL)
	if err != nil {
		return nil, fmt.Errorf("invalid redis URL: %w", err)
	}

	host := u.Hostname()
	port := u.Port()
	if port == "" {
		port = "6379"
	}

	password := ""
	if u.User != nil {
		password, _ = u.User.Password()
	}

	db := 0
	if len(u.Path) > 1 {
		db, _ = strconv.Atoi(u.Path[1:])
	}

	conn := &SimpleConn{
		addr:     fmt.Sprintf("%s:%s", host, port),
		password: password,
		db:       db,
	}

	client := &Client{
		conn:    conn,
		address: conn.addr,
	}

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx); err != nil {
		log.Warn().Err(err).Str("address", conn.addr).Msg("Redis connection test failed")
		// Don't fail - we'll retry on each request
	} else {
		log.Info().Str("address", conn.addr).Msg("Redis connected")
	}

	return client, nil
}

// HGet gets a hash field value
func (c *Client) HGet(ctx context.Context, key, field string) (string, error) {
	result, err := c.conn.Do(ctx, "HGET", key, field)
	if err != nil {
		return "", err
	}
	if result == nil {
		return "", nil
	}
	if s, ok := result.(string); ok {
		return s, nil
	}
	if b, ok := result.([]byte); ok {
		return string(b), nil
	}
	return "", fmt.Errorf("unexpected result type: %T", result)
}

// Ping tests the connection
func (c *Client) Ping(ctx context.Context) error {
	result, err := c.conn.Do(ctx, "PING")
	if err != nil {
		return err
	}
	if s, ok := result.(string); ok && s == "PONG" {
		return nil
	}
	return fmt.Errorf("unexpected PING response: %v", result)
}

// Close closes the connection
func (c *Client) Close() error {
	return c.conn.Close()
}

// Do executes a Redis command (for SimpleConn)
func (c *SimpleConn) Do(ctx context.Context, args ...interface{}) (interface{}, error) {
	// Use a simple TCP connection for each command
	conn, err := net.DialTimeout("tcp", c.addr, 5*time.Second)
	if err != nil {
		return nil, fmt.Errorf("connect failed: %w", err)
	}
	defer conn.Close()

	// Set deadline from context
	if deadline, ok := ctx.Deadline(); ok {
		conn.SetDeadline(deadline)
	}

	// AUTH if password set
	if c.password != "" {
		if err := c.sendCommand(conn, "AUTH", c.password); err != nil {
			return nil, err
		}
		if _, err := c.readResponse(conn); err != nil {
			return nil, fmt.Errorf("AUTH failed: %w", err)
		}
	}

	// SELECT database if not 0
	if c.db != 0 {
		if err := c.sendCommand(conn, "SELECT", strconv.Itoa(c.db)); err != nil {
			return nil, err
		}
		if _, err := c.readResponse(conn); err != nil {
			return nil, fmt.Errorf("SELECT failed: %w", err)
		}
	}

	// Send the actual command
	if err := c.sendCommand(conn, args...); err != nil {
		return nil, err
	}

	return c.readResponse(conn)
}

func (c *SimpleConn) sendCommand(conn net.Conn, args ...interface{}) error {
	// RESP protocol: *<num args>\r\n$<len>\r\n<arg>\r\n...
	cmd := fmt.Sprintf("*%d\r\n", len(args))
	for _, arg := range args {
		s := fmt.Sprintf("%v", arg)
		cmd += fmt.Sprintf("$%d\r\n%s\r\n", len(s), s)
	}
	_, err := conn.Write([]byte(cmd))
	return err
}

func (c *SimpleConn) readResponse(conn net.Conn) (interface{}, error) {
	reader := bufio.NewReader(conn)
	line, err := reader.ReadString('\n')
	if err != nil {
		return nil, err
	}
	line = line[:len(line)-2] // Strip \r\n

	switch line[0] {
	case '+': // Simple string
		return line[1:], nil
	case '-': // Error
		return nil, fmt.Errorf("redis error: %s", line[1:])
	case ':': // Integer
		n, _ := strconv.ParseInt(line[1:], 10, 64)
		return n, nil
	case '$': // Bulk string
		length, _ := strconv.Atoi(line[1:])
		if length == -1 {
			return nil, nil // Nil
		}
		data := make([]byte, length+2)
		_, err := reader.Read(data)
		if err != nil {
			return nil, err
		}
		return string(data[:length]), nil
	case '*': // Array
		count, _ := strconv.Atoi(line[1:])
		if count == -1 {
			return nil, nil
		}
		result := make([]interface{}, count)
		for i := 0; i < count; i++ {
			result[i], err = c.readResponse(conn)
			if err != nil {
				return nil, err
			}
		}
		return result, nil
	}
	return nil, fmt.Errorf("unknown response type: %s", line)
}

// Close for SimpleConn - connection-per-request, nothing to close
func (c *SimpleConn) Close() error {
	return nil
}
