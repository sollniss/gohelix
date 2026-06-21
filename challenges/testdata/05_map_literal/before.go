package httperr

func messages() map[int]string {
	m := make(map[int]string)
	m[400] = "bad request"
	m[404] = "not found"
	m[500] = "server error"
	return m
}
