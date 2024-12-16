package clients

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"net"
	"time"
)

type KapingerDNSClient struct {
	volume   int
	interval time.Duration
}

func NewKapingerDNSClient(volume int, interval time.Duration) *KapingerDNSClient {
	return &KapingerDNSClient{
		interval: time.Duration(interval),
		volume:   volume,
	}
}

func (k *KapingerDNSClient) MakeRequests(ctx context.Context) error {
	ticker := time.NewTicker(k.interval)
	for {
		select {
		case <-ctx.Done():
			log.Printf("DNS client context done")
			return nil
		case <-ticker.C:
			go func() {
				for i := 0; i < k.volume; i++ {
					domain := randomString(20) + ".test"

					ips, err := net.LookupIP(domain)
					if err != nil {
						fmt.Printf("dns client: could not get IPs: %v\n", err)
						return
					}
					log.Printf("dns client: resolved %s to %s\n", domain, ips)
				}
			}()
		}
	}
}

func randomString(n int) string {
	var letters = []rune("abcdefghijklmnopqrstuvwxyz")
	b := make([]rune, n)

	rand.New(rand.NewSource(time.Now().UnixNano()))

	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}

	return string(b)
}
