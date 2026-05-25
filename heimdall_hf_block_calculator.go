// go run heimdall_hf_block_calculator.go -rpc="https://tendermint-api.polygon.technology" -target="2026-06-01T14:00:00Z" -avg=1.30
// go run heimdall_hf_block_calculator.go help

package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
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

func main() {
	fs := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	rpcURL := fs.String("rpc", "", "Heimdall Tendermint RPC base URL")
	baseAlias := fs.String("base", "", "Alias for -rpc")
	targetStr := fs.String("target", "", "Target UTC time in RFC3339/RFC3339Nano format")
	avgSecs := fs.Float64("avg", 0, "Average block time in seconds")
	timeout := fs.Duration("timeout", 15*time.Second, "HTTP request timeout")
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage:\n  go run heimdall_hf_block_calculator.go -rpc=<heimdall-rpc-url> -target=<utc-time> -avg=<seconds> [options]\n  go run heimdall_hf_block_calculator.go help\n\n")
		fmt.Fprintln(fs.Output(), "Predicts the Heimdall block height for a target UTC timestamp using the current chain head and an average block time.")
		fmt.Fprintln(fs.Output(), "\nRequired:")
		fmt.Fprintln(fs.Output(), "  -rpc string")
		fmt.Fprintln(fs.Output(), "        Heimdall Tendermint RPC base URL for the network being scheduled")
		fmt.Fprintln(fs.Output(), "  -target string")
		fmt.Fprintln(fs.Output(), "        Target UTC time in RFC3339/RFC3339Nano format, for example 2026-06-01T14:00:00Z")
		fmt.Fprintln(fs.Output(), "  -avg float")
		fmt.Fprintln(fs.Output(), "        Average block time in seconds, usually copied from heimdall_average_blocktime_calculator.go")
		fmt.Fprintln(fs.Output(), "\nOptions:")
		fs.PrintDefaults()
		fmt.Fprintln(fs.Output(), "\nExamples:")
		fmt.Fprintln(fs.Output(), "  go run heimdall_hf_block_calculator.go -rpc=https://tendermint-api.polygon.technology -target=2026-06-01T14:00:00Z -avg=1.30")
		fmt.Fprintln(fs.Output(), "  go run heimdall_hf_block_calculator.go -rpc=https://tendermint-api-amoy.polygon.technology -target=2026-06-01T14:00:00Z -avg=1.25")
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
	missing := missingRequiredFlags(fs, *rpcURL, *targetStr)
	if len(missing) > 0 {
		failf("missing required options: %s\n\nRequired inputs:\n  -rpc    Heimdall Tendermint RPC base URL for the network being scheduled.\n  -target Hardfork target time in UTC RFC3339 format, for example 2026-06-01T14:00:00Z.\n  -avg    Positive average Heimdall block time in seconds, usually copied from `heimdall_average_blocktime_calculator.go`.\n\nExamples:\n  mainnet: go run heimdall_hf_block_calculator.go -rpc=https://tendermint-api.polygon.technology -target=2026-06-01T14:00:00Z -avg=1.30\n  amoy   : go run heimdall_hf_block_calculator.go -rpc=https://tendermint-api-amoy.polygon.technology -target=2026-06-01T14:00:00Z -avg=1.25\n\nRun `go run heimdall_hf_block_calculator.go help` for all options.", strings.Join(missing, ", "))
	}
	if *avgSecs <= 0 {
		failf("invalid -avg: %.6f\n\nProvide a positive average Heimdall block time in seconds, usually copied from `heimdall_average_blocktime_calculator.go`.\nExample:\n  -avg=1.30\n\nRun `go run heimdall_hf_block_calculator.go help` for all options.", *avgSecs)
	}
	targetTime, err := parseTarget(*targetStr)
	if err != nil {
		failf("parse target time: %v", err)
	}

	ctx := context.Background()
	httpc := &http.Client{Timeout: *timeout}

	latestHeight, latestTime, _, err := getLatest(ctx, httpc, *rpcURL)
	if err != nil {
		failf("get latest: %v", err)
	}

	delta := targetTime.Sub(latestTime)
	deltaSeconds := delta.Seconds()
	blocksFloat := deltaSeconds / *avgSecs
	blocksRounded := int64(math.Round(blocksFloat))
	predicted := latestHeight + blocksRounded
	if predicted < 0 {
		predicted = 0
	}

	sign := "+"
	if delta < 0 {
		sign = "-"
	}

	fmt.Printf("Current block : %s at %s\n", withCommasInt64(latestHeight), latestTime.UTC().Format(time.RFC3339))
	fmt.Printf("Target time   : %s (UTC)\n", targetTime.Format(time.RFC3339))
	fmt.Printf("Avg block     : %.6f s\n", *avgSecs)
	fmt.Printf("\nΔtime         : %s%s (%s s)\n", sign, elapsedDHMS(delta), withCommasInt64(int64(math.Abs(deltaSeconds))))
	fmt.Printf("Estimated Δblk: %s%s (rounded) — %.3f (exact)\n", sign, withCommasInt64(absInt64(blocksRounded)), blocksFloat)
	fmt.Printf("\nPredicted block at target:\n")
	fmt.Printf("  height      : %s\n", withCommasInt64(predicted))
}

func isHelpCommand(arg string) bool {
	return arg == "help" || arg == "-help" || arg == "--help" || arg == "-h"
}

func missingRequiredFlags(fs *flag.FlagSet, rpcURL, targetStr string) []string {
	var missing []string
	if strings.TrimSpace(rpcURL) == "" {
		missing = append(missing, "-rpc")
	}
	if strings.TrimSpace(targetStr) == "" {
		missing = append(missing, "-target")
	}
	if !flagWasProvided(fs, "avg") {
		missing = append(missing, "-avg")
	}
	return missing
}

func flagWasProvided(fs *flag.FlagSet, name string) bool {
	provided := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == name {
			provided = true
		}
	})
	return provided
}

func parseTarget(s string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t.UTC(), nil
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.UTC(), nil
	}
	return time.Time{}, fmt.Errorf("unsupported time format %q (use RFC3339/RFC3339Nano, e.g. 2026-06-01T14:00:00Z)", s)
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

func elapsedDHMS(d time.Duration) string {
	if d < 0 {
		d = -d
	}
	totalSec := int64(d.Seconds())
	dd := totalSec / 86400
	r := totalSec % 86400
	hh := r / 3600
	r %= 3600
	mm := r / 60
	ss := r % 60
	return fmt.Sprintf("%dd %dh %dm %ds", dd, hh, mm, ss)
}

func withCommasInt64(v int64) string {
	if v < 0 {
		return "-" + withCommasUint64(uint64(-v))
	}
	return withCommasUint64(uint64(v))
}

func withCommasUint64(u uint64) string {
	s := fmt.Sprintf("%d", u)
	n := len(s)
	if n <= 3 {
		return s
	}
	var b strings.Builder
	pre := n % 3
	if pre == 0 {
		pre = 3
	}
	b.WriteString(s[:pre])
	for i := pre; i < n; i += 3 {
		b.WriteByte(',')
		b.WriteString(s[i : i+3])
	}
	return b.String()
}

func absInt64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}

func failf(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "error: "+format+"\n", a...)
	os.Exit(1)
}
