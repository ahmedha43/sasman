package main

import (
	"context"
	"crypto/rand"
	"flag"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	layehRadius "layeh.com/radius"
	"layeh.com/radius/rfc2759"
	"layeh.com/radius/rfc2865"
	"layeh.com/radius/vendors/microsoft"
)

type result struct {
	code    layehRadius.Code
	elapsed time.Duration
	err     error
}

func main() {
	addr := flag.String("addr", "127.0.0.1:1812", "RADIUS server address")
	secret := flag.String("secret", "testing123", "NAS shared secret")
	username := flag.String("user", "test", "RADIUS username")
	password := flag.String("pass", "test", "RADIUS password")
	mode := flag.String("mode", "mschapv2", "auth mode: pap or mschapv2")
	count := flag.Int("n", 100, "number of Access-Request packets")
	concurrency := flag.Int("c", 10, "parallel workers")
	timeout := flag.Duration("timeout", 3*time.Second, "per-request timeout")
	retry := flag.Duration("retry", 0, "client retry interval, 0 disables retry")
	flag.Parse()

	if *count <= 0 || *concurrency <= 0 {
		fmt.Println("n and c must be greater than zero")
		return
	}

	jobs := make(chan int)
	results := make(chan result, *count)
	var started atomic.Int64

	client := &layehRadius.Client{
		Retry:           *retry,
		MaxPacketErrors: 3,
	}

	start := time.Now()
	var wg sync.WaitGroup
	for i := 0; i < *concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for jobID := range jobs {
				started.Add(1)
				results <- sendAccessRequest(client, *addr, []byte(*secret), *username, *password, *mode, jobID, *timeout)
			}
		}(i)
	}

	for i := 0; i < *count; i++ {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
	close(results)

	var accepts, rejects, challenges, other, failed int
	latencies := make([]time.Duration, 0, *count)
	for res := range results {
		if res.err != nil {
			failed++
			continue
		}
		latencies = append(latencies, res.elapsed)
		switch res.code {
		case layehRadius.CodeAccessAccept:
			accepts++
		case layehRadius.CodeAccessReject:
			rejects++
		case layehRadius.CodeAccessChallenge:
			challenges++
		default:
			other++
		}
	}

	total := time.Since(start)
	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })

	fmt.Printf("RADIUS load test complete\n")
	fmt.Printf("  target:      %s\n", *addr)
	fmt.Printf("  mode:        %s\n", *mode)
	fmt.Printf("  sent:        %d\n", *count)
	fmt.Printf("  concurrency: %d\n", *concurrency)
	fmt.Printf("  duration:    %s\n", total.Round(time.Millisecond))
	fmt.Printf("  throughput:  %.1f req/s\n", float64(*count)/total.Seconds())
	fmt.Printf("  accept:      %d\n", accepts)
	fmt.Printf("  reject:      %d\n", rejects)
	fmt.Printf("  challenge:   %d\n", challenges)
	fmt.Printf("  other:       %d\n", other)
	fmt.Printf("  failed:      %d\n", failed)
	if len(latencies) > 0 {
		fmt.Printf("  latency avg: %s\n", avg(latencies).Round(time.Microsecond))
		fmt.Printf("  latency p50: %s\n", percentile(latencies, 50).Round(time.Microsecond))
		fmt.Printf("  latency p95: %s\n", percentile(latencies, 95).Round(time.Microsecond))
		fmt.Printf("  latency max: %s\n", latencies[len(latencies)-1].Round(time.Microsecond))
	}
}

func sendAccessRequest(client *layehRadius.Client, addr string, secret []byte, username, password, mode string, jobID int, timeout time.Duration) result {
	packet, err := buildAccessRequest(secret, username, password, mode, jobID)
	if err != nil {
		return result{err: err}
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	start := time.Now()
	response, err := client.Exchange(ctx, packet, addr)
	if err != nil {
		return result{elapsed: time.Since(start), err: err}
	}
	return result{code: response.Code, elapsed: time.Since(start)}
}

func buildAccessRequest(secret []byte, username, password, mode string, jobID int) (*layehRadius.Packet, error) {
	packet := layehRadius.New(layehRadius.CodeAccessRequest, secret)
	rfc2865.UserName_AddString(packet, username)
	rfc2865.ServiceType_Add(packet, rfc2865.ServiceType_Value_FramedUser)
	rfc2865.NASIdentifier_AddString(packet, "sasman-loadtest")
	rfc2865.CallingStationID_AddString(packet, fmt.Sprintf("loadtest-%06d", jobID))

	switch mode {
	case "pap":
		return packet, rfc2865.UserPassword_AddString(packet, password)
	case "mschapv2":
		if err := addMSCHAPv2(packet, username, password, byte(jobID%255)); err != nil {
			return nil, err
		}
		return packet, nil
	default:
		return nil, fmt.Errorf("unknown mode %q", mode)
	}
}

func addMSCHAPv2(packet *layehRadius.Packet, username, password string, ident byte) error {
	var challenge [16]byte
	var peerChallenge [16]byte
	if _, err := rand.Read(challenge[:]); err != nil {
		return err
	}
	if _, err := rand.Read(peerChallenge[:]); err != nil {
		return err
	}

	ntResponse, err := rfc2759.GenerateNTResponse(challenge[:], peerChallenge[:], []byte(username), []byte(password))
	if err != nil {
		return err
	}

	mschapResponse := make([]byte, 50)
	mschapResponse[0] = ident
	copy(mschapResponse[2:18], peerChallenge[:])
	copy(mschapResponse[26:50], ntResponse)

	if err := microsoft.MSCHAPChallenge_Add(packet, challenge[:]); err != nil {
		return err
	}
	return microsoft.MSCHAP2Response_Add(packet, mschapResponse)
}

func avg(values []time.Duration) time.Duration {
	var total time.Duration
	for _, value := range values {
		total += value
	}
	return total / time.Duration(len(values))
}

func percentile(values []time.Duration, p int) time.Duration {
	if len(values) == 0 {
		return 0
	}
	index := (len(values)*p + 99) / 100
	if index < 1 {
		index = 1
	}
	if index > len(values) {
		index = len(values)
	}
	return values[index-1]
}
