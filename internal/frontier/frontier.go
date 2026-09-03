package frontier

import (
	"net/url"
	"strings"
)

type Task struct {
	URL      *url.URL
	Attempts int // ignored by in memory implementation
}

type Frontier interface {
	Enqueue(t Task)
	Next() (t Task, ok bool)
	Ack(t Task)
	Fail(t Task)
	Close()
}

func Canonical(u *url.URL) *url.URL {
	c := *u
	c.Host = strings.ToLower(c.Host)
	if c.Path == "" {
		c.Path = "/"
	}
	return &c
}
