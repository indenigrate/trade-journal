package main

import (
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func main() {
	sub := os.Args[1]
	claims := jwt.MapClaims{
		"sub":  sub,
		"role": "trader",
		"iat":  time.Now().Unix(),
		"exp":  time.Now().Add(time.Hour).Unix(),
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	s, _ := t.SignedString([]byte("97791d4db2aa5f689c3cc39356ce35762f0a73aa70923039d8ef72a2840a1b02"))
	fmt.Print(s)
}
