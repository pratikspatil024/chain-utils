package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultLookbacks = "10000,100000,1000000,1500000"
)

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
	fs := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	rpcURL := fs.String("rpc", "", "Heimdall Tendermint RPC base URL")
	baseAlias := fs.String("base", "", "Alias for -rpc")
	lookbacksFlag := fs.String("lookbacks", defaultLookbacks, "Comma-separated block lookbacks to average")
	timeout := fs.Duration("timeout", 15*time.Second, "HTTP request timeout")
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage:\n  go run heimdall_average_blocktime_calculator.go -rpc=<heimdall-rpc-url> [options]\n  go run heimdall_average_blocktime_calculator.go help\n\n")
		fmt.Fprintln(fs.Output(), "Calculates Heimdall average block time from the latest block back to each configured lookback height.")
		fmt.Fprintln(fs.Output(), "\nRequired:")
		fmt.Fprintln(fs.Output(), "  -rpc string")
		fmt.Fprintln(fs.Output(), "        Heimdall Tendermint RPC base URL for the network being scheduled")
		fmt.Fprintln(fs.Output(), "\nOptions:")
		fs.PrintDefaults()
		fmt.Fprintln(fs.Output(), "\nExamples:")
		fmt.Fprintln(fs.Output(), "  go run heimdall_average_blocktime_calculator.go -rpc=https://tendermint-api.polygon.technology")
		fmt.Fprintln(fs.Output(), "  go run heimdall_average_blocktime_calculator.go -rpc=https://tendermint-api-amoy.polygon.technology -lookbacks=10000,50000,100000")
	}
	if len(os.Args) > 1 && isHelpCommand(os.Args[1]) {
		fs.Usage()
		return
	}
	if err := fs.Parse(os.Args[1:]); err != nil {
		failf("%v", err)
	}
	if strings.TrimSpace(*baseAlias) != "" {
		*rpcURL = *baseAlias
	}
	if strings.TrimSpace(*rpcURL) == "" {
		failf("missing required -rpc\n\nProvide the Heimdall Tendermint RPC base URL for the network being measured.\nExamples:\n  mainnet: -rpc=https://tendermint-api.polygon.technology\n  amoy   : -rpc=https://tendermint-api-amoy.polygon.technology\n\nRun `go run heimdall_average_blocktime_calculator.go help` for all options.")
	}
	lookbacks, err := parseLookbacks(*lookbacksFlag)
	if err != nil {
		failf("parse lookbacks: %v", err)
	}

	ctx := context.Background()
	httpc := &http.Client{Timeout: *timeout}

	latestHeight, latestTime, earliestHeight, err := getLatest(ctx, httpc, *rpcURL)
	if err != nil {
		failf("get latest: %v", err)
	}

	fmt.Printf("Current block: %d at %s (earliest available: %d)\n\n",
		latestHeight, latestTime.Format(time.RFC3339Nano), earliestHeight)

	for _, lb := range lookbacks {
		target := latestHeight - lb
		if target < earliestHeight {
			fmt.Printf("Δ%-9d SKIP  target height %d < earliest available %d\n", lb, target, earliestHeight)
			continue
		}
		t0, err := getBlockTime(ctx, httpc, *rpcURL, target)
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

func isHelpCommand(arg string) bool {
	return arg == "help" || arg == "-help" || arg == "--help" || arg == "-h"
}

func parseLookbacks(raw string) ([]int64, error) {
	parts := strings.Split(raw, ",")
	lookbacks := make([]int64, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		lookback, err := strconv.ParseInt(part, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid lookback %q: %w", part, err)
		}
		if lookback <= 0 {
			return nil, fmt.Errorf("lookback must be greater than zero")
		}
		lookbacks = append(lookbacks, lookback)
	}
	if len(lookbacks) == 0 {
		return nil, fmt.Errorf("at least one lookback is required")
	}
	return lookbacks, nil
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

func failf(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "error: "+format+"\n", a...)
	os.Exit(1)
}
