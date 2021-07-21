package main

import (
	"flag"
	"github.com/iberryful/sproxy/pkg/client"
	"github.com/iberryful/sproxy/pkg/log"
	"github.com/pkg/profile"
	"time"
)

var (
	secret        string
	remoteAddr    string
	listenAddr    string
	logLevel      string
	poolSize      int
	enableProfile bool
	timeout       time.Duration
)

func init() {
	//runtime.GOMAXPROCS(32)
	flag.StringVar(&secret, "s", "secret", "secret")
	flag.StringVar(&remoteAddr, "r", "127.0.0.1:7443", "remote addr")
	flag.StringVar(&listenAddr, "l", "127.0.0.1:2080", "local listen addr")
	flag.StringVar(&logLevel, "v", "info", "log level")
	flag.IntVar(&poolSize, "c", 32, "connection pool size")
	flag.BoolVar(&enableProfile, "p", false, "enable profile")
	flag.DurationVar(&timeout, "t", 1*time.Minute, "timeout for idle connection in pool")
	flag.Parse()
	log.SetLevel(logLevel)
}

func main() {
	if enableProfile {
		defer profile.Start(profile.CPUProfile, profile.ProfilePath(".")).Stop()
	}
	o := &client.ClientOption{
		ListenAddr: listenAddr,
		RemoteAddr: remoteAddr,
		Secret:     secret,
		PoolSize:   poolSize,
		Timeout:    timeout,
	}
	c := client.New(o)
	log.Error(c.ListenAndServe())
}
