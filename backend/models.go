package main

type URL struct {
	ID         string      `gorm:"primary_key"`
	URL        string      `gorm:"not null"`
	ResultChan chan string `gorm:"-"` // Used for communicating results
}

type ReadRequest struct {
	ID         string
	ResultChan chan map[string]string
}
