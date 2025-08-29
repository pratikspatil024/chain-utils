package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

const defaultBase = "https://tendermint-api.polygon.technology"

type statusResp struct {
	Result struct {
		SyncInfo struct {
			LatestBlockHeight string `json:"latest_block_height"`
			LatestBlockTime   string `json:"latest_block_time"`
			EarliestBlockH    string `json:"earliest_block_height"`
		} `json:"sync_info"`
	} `json:"result"`
}

type blockResp struct {
	Result struct {
		Block struct {
			Header struct {
				Height string `json:"height"`
				Time   string `json:"time"`
			} `json:"header"`
		} `json:"block"`
	} `json:"result"`
}

func main() {
	base := flag.String("base", defaultBase, "Base URL for the Tendermint RPC-compatible API")
	timeout := flag.Duration("timeout", 15*time.Second, "HTTP request timeout")
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	httpc := &http.Client{Timeout: *timeout}

	latestHeight, latestTime, earliestHeight, err := getLatest(ctx, httpc, *base)
	if err != nil {
		panic(fmt.Errorf("get latest: %w", err))
	}

	fmt.Printf("Current block: %d at %s (earliest available: %d)\n\n",
		latestHeight, latestTime.Format(time.RFC3339Nano), earliestHeight)

	lookbacks := []int64{10_000, 100_000, 1_000_000, 1_500_000}
	for _, lb := range lookbacks {
		target := latestHeight - lb
		if target < earliestHeight {
			fmt.Printf("Δ%-9d SKIP  target height %d < earliest available %d\n", lb, target, earliestHeight)
			continue
		}
		t0, err := getBlockTime(ctx, httpc, *base, target)
		if err != nil {
			fmt.Printf("Δ%-9d ERROR fetching height %d: %v\n", lb, target, err)
			continue
		}
		elapsed := latestTime.Sub(t0)                 // total elapsed
		avgSeconds := elapsed.Seconds() / float64(lb) // average seconds per block

		fmt.Printf("Δ%-9d from height %-10d to %-10d\n", lb, target, latestHeight)
		fmt.Printf("  elapsed    : %s\n", formatElapsed(elapsed))
		fmt.Printf("  avg block  : %.6f s/block  (%.3f ms)\n\n", avgSeconds, avgSeconds*1000.0)
	}
}

func formatElapsed(d time.Duration) string {
	if d < 0 {
		d = -d
	}
	days := d / (24 * time.Hour)
	d -= days * 24 * time.Hour
	hours := d / time.Hour
	d -= hours * time.Hour
	mins := d / time.Minute
	d -= mins * time.Minute
	secs := d / time.Second

	return fmt.Sprintf("%dd %dh %dm %ds", days, hours, mins, secs)
}

func getLatest(ctx context.Context, c *http.Client, base string) (height int64, t time.Time, earliest int64, err error) {
	u := base + "/status"
	var sr statusResp
	if err = getJSON(ctx, c, u, &sr); err != nil {
		return
	}
	h, err1 := strconv.ParseInt(sr.Result.SyncInfo.LatestBlockHeight, 10, 64)
	if err1 != nil {
		err = fmt.Errorf("parse latest height: %w", err1)
		return
	}
	earliest, err1 = strconv.ParseInt(sr.Result.SyncInfo.EarliestBlockH, 10, 64)
	if err1 != nil {
		err = fmt.Errorf("parse earliest height: %w", err1)
		return
	}
	t, err1 = time.Parse(time.RFC3339Nano, sr.Result.SyncInfo.LatestBlockTime)
	if err1 != nil {
		err = fmt.Errorf("parse latest time: %w", err1)
		return
	}
	height = h
	return
}

func getBlockTime(ctx context.Context, c *http.Client, base string, height int64) (time.Time, error) {
	u := fmt.Sprintf("%s/block?height=%d", base, height)
	var br blockResp
	if err := getJSON(ctx, c, u, &br); err != nil {
		return time.Time{}, err
	}
	ts := br.Result.Block.Header.Time
	if ts == "" {
		return time.Time{}, errors.New("empty block time")
	}
	t, err := time.Parse(time.RFC3339Nano, ts)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse block time: %w", err)
	}
	return t, nil
}

func getJSON(ctx context.Context, c *http.Client, url string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP %d for %s", resp.StatusCode, url)
	}
	dec := json.NewDecoder(resp.Body)
	return dec.Decode(out)
}

