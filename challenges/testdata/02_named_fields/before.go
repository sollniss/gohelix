package main

type Server struct {
	Host    string
	Port    int
	Debug   bool
	Timeout int
}

func newServers() []*Server {
	return []*Server{
		{"localhost", 8080, true, 30},
		{"0.0.0.0", 9090, false, 60},
		{"127.0.0.1", 3000, true, 15},
	}
}
