/*
How to run?
`go run heimdall_hf_block_calculator.go`

What does it do?
TLDR: It predicts the **future block height** for a given target UTC time and average block time.
1. Fetch the current block height and timestamp.
2. Parse the hardcoded target timestamp.
3. Calculate the time difference (delta) between now and the target.
4. Divide delta by the average block time to estimate number of blocks.
5. Add blocks to current height -> predicted future block height.
6. Print the predicted height and time delta (in days, hours, minutes, seconds).
*/

package main

import (
	"context"
	"encoding/json"
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

func main() {
	base := flag.String("base", defaultBase, "Base URL for the Tendermint RPC-compatible API")
	timeout := flag.Duration("timeout", 15*time.Second, "HTTP request timeout")
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	httpc := &http.Client{Timeout: *timeout}

	// Get current height + time
	latestHeight, latestTime, _, err := getLatest(ctx, httpc, *base)
	if err != nil {
		panic(fmt.Errorf("get latest: %w", err))
	}
	fmt.Printf("Current block: %d at %s\n\n",
		latestHeight, latestTime.Format(time.RFC3339Nano))

	// --- FUTURE BLOCK CALCULATION ---
	// Hardcode arguments here:
	targetTimeStr := "2025-09-16T14:00:00.00000000Z"
	avgBlockTime := 1.30 // seconds

	targetTime, err := time.Parse(time.RFC3339Nano, targetTimeStr)
	if err != nil {
		panic(fmt.Errorf("parse target time: %w", err))
	}

	delta := targetTime.Sub(latestTime)
	if delta < 0 {
		fmt.Printf("Target time %s is in the past relative to latest block.\n", targetTime.Format(time.RFC3339))
		return
	}

	blocksToAdd := int64(delta.Seconds() / avgBlockTime)
	predicted := latestHeight + blocksToAdd

	fmt.Println("Future block prediction:")
	fmt.Printf("  target time     : %s\n", targetTime.Format(time.RFC3339))
	fmt.Printf("  avg block time  : %.2f s\n", avgBlockTime)
	fmt.Printf("  time delta      : %dd %dh %dm %ds\n", int(delta.Hours())/24, int(delta.Hours())%24, int(delta.Minutes())%60, int(delta.Seconds())%60)
	fmt.Printf("  blocks to add   : %d\n", blocksToAdd)
	fmt.Printf("  predicted height: %d\n", predicted)
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

