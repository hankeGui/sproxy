package main

import (
	"flag"
	"github.com/iberryful/sproxy/pkg/log"
	"github.com/iberryful/sproxy/pkg/server"
	"github.com/pkg/profile"
)

var (
	listenAddr    string
	logLevel      string
	secret        string
	key           string
	crt           string
	enableProfile bool
)

func init() {
	flag.StringVar(&listenAddr, "l", "127.0.0.1:7443", "listen address")
	flag.StringVar(&secret, "s", "secret", "secret")
	flag.StringVar(&logLevel, "v", "info", "log level")
	flag.StringVar(&key, "k", "examples/key.pem", "server private key")
	flag.StringVar(&crt, "c", "examples/cert.pem", "server certificate")
	flag.BoolVar(&enableProfile, "p", false, "enable profile")
	flag.Parse()
	log.SetLevel(logLevel)
}

func main() {
	if enableProfile {
		defer profile.Start(profile.CPUProfile, profile.ProfilePath(".")).Stop()
	}
	o := &server.ServerOption{
		KeyPath: key,
		CrtPath: crt,
		Secret:  secret,
		Listen:  listenAddr,
	}
	s, err := server.NewServer(o)
	if err != nil {
		log.Fatal(err)
	}
	log.Error(s.Start())
}
