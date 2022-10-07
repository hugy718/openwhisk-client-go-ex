package main

import (
	"log"
	"sync"
	"time"
)

// rate: req/s
// duration: mins
func test_time(rate int, duration float64) {
	id := 0
	var wg sync.WaitGroup
	inter_arrival := 1.0 / float64(rate)
	start_time := time.Now().UnixMicro()
	for t := 0.0; t < duration*60.0; t += inter_arrival {
		wg.Add(1)
		now := time.Now().UnixMicro()
		if start_time+int64(t*1e6) > now {
			sleep_duration := start_time + int64(t*1e6) - now
			time.Sleep(time.Duration(sleep_duration) * time.Microsecond)
			log.Printf("sleep time: %v", sleep_duration)
		}
		go func(id int) {
			defer wg.Done()
			log.Printf("echo time: %v", time.Now().UnixMicro())
			time.Sleep(time.Duration(1 * time.Second))
		}(id)
		id++
	}
	wg.Wait()
}

func main() {
	test_time(200, 0.1)
}
