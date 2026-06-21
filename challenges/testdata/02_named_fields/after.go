package main

type Server struct {
	Host    string
	Port    int
	Debug   bool
	Timeout int
}

func newServers() []*Server {
	return []*Server{
		{Host: "localhost", Port: 8080, Debug: true, Timeout: 30},
		{Host: "0.0.0.0", Port: 9090, Debug: false, Timeout: 60},
		{Host: "127.0.0.1", Port: 3000, Debug: true, Timeout: 15},
	}
}
