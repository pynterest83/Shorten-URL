package main

import (
	"math/rand"
)

// URLData represents the structure of a URL record in the database
type URLData struct {
	ID  string `db:"id"`
	URL string `db:"url"`
}

// makeID generates a random string ID of a given length
func makeID(length int) string {
	const characters = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	result := make([]byte, length)
	for i := 0; i < length; i++ {
		result[i] = characters[rand.Intn(len(characters))]
	}
	return string(result)
}
