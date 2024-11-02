package main

import (
	"franklyner/gores/middleware"
	"os"
	"testing"
)

func TestShowEnv(t *testing.T) {
	os.Setenv(middleware.EnvPATH, "/env")
	os.Setenv(middleware.EnvQUERY, "test=true")
	main()
}
