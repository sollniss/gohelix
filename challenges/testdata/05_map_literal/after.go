package httperr

func messages() map[int]string {
	return map[int]string{
		400: "bad request",
		404: "not found",
		500: "server error",
	}
}
