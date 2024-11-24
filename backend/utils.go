package main

import (
	"sync"
)

var idProcessing sync.Map

var channelPool = sync.Pool{
	New: func() interface{} {
		return make(chan map[string]string, 1) // Buffer size 1 to avoid blocking
	},
}

func getOrCreateChannel(id string) (chan string, bool) {
	ch, loaded := idProcessing.LoadOrStore(id, make(chan string, 1)) // Channel with buffer size 1
	return ch.(chan string), loaded
}

func notifyChannel(id string, data string) {
	if ch, ok := idProcessing.Load(id); ok {
		ch.(chan string) <- data
	}
}

func closeChannel(id string) {
	if ch, ok := idProcessing.LoadAndDelete(id); ok {
		close(ch.(chan string))
	}
}
