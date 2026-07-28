// go run heimdall_block_time_estimator.go -rpc="https://tendermint-api.polygon.technology" -target=50185000 -avg=1.30
// go run heimdall_block_time_estimator.go help

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
	targetStr := fs.String("target", "", "Target Heimdall block height")
	avgSecs := fs.Float64("avg", 0, "Average block time in seconds")
	timeout := fs.Duration("timeout", 15*time.Second, "HTTP request timeout")
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage:\n  go run heimdall_block_time_estimator.go -rpc=<heimdall-rpc-url> -target=<block-height> -avg=<seconds> [options]\n  go run heimdall_block_time_estimator.go help\n\n")
		fmt.Fprintln(fs.Output(), "Predicts when Heimdall will reach a target block height using the current chain head and a supplied average block time.")
		fmt.Fprintln(fs.Output(), "\nRequired:")
		fmt.Fprintln(fs.Output(), "  -rpc string")
		fmt.Fprintln(fs.Output(), "        Heimdall Tendermint RPC base URL for the network being scheduled")
		fmt.Fprintln(fs.Output(), "  -target string")
		fmt.Fprintln(fs.Output(), "        Target Heimdall block height, for example 50185000")
		fmt.Fprintln(fs.Output(), "  -avg float")
		fmt.Fprintln(fs.Output(), "        Average block time in seconds, usually copied from heimdall_average_blocktime_calculator.go")
		fmt.Fprintln(fs.Output(), "\nOptions:")
		fs.PrintDefaults()
		fmt.Fprintln(fs.Output(), "\nExamples:")
		fmt.Fprintln(fs.Output(), "  go run heimdall_block_time_estimator.go -rpc=https://tendermint-api.polygon.technology -target=50185000 -avg=1.30")
		fmt.Fprintln(fs.Output(), "  go run heimdall_block_time_estimator.go -rpc=https://tendermint-api-amoy.polygon.technology -target=13143851 -avg=1.25")
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
		failf("missing required options: %s\n\nRequired inputs:\n  -rpc    Heimdall Tendermint RPC base URL for the network being scheduled.\n  -target Target Heimdall block height, for example 50185000.\n  -avg    Positive average Heimdall block time in seconds, usually copied from `heimdall_average_blocktime_calculator.go`.\n\nExample:\n  go run heimdall_block_time_estimator.go -rpc=https://tendermint-api.polygon.technology -target=50185000 -avg=1.30\n\nRun `go run heimdall_block_time_estimator.go help` for all options.", strings.Join(missing, ", "))
	}
	if *avgSecs <= 0 {
		failf("invalid -avg: %.6f\n\nProvide a positive average Heimdall block time in seconds, usually copied from `heimdall_average_blocktime_calculator.go`.\nExample:\n  -avg=1.30\n\nRun `go run heimdall_block_time_estimator.go help` for all options.", *avgSecs)
	}
	targetHeight, err := parseTargetHeight(*targetStr)
	if err != nil {
		failf("parse target height: %v", err)
	}

	ctx := context.Background()
	httpc := &http.Client{Timeout: *timeout}
	latestHeight, latestTime, _, err := getLatest(ctx, httpc, *rpcURL)
	if err != nil {
		failf("get latest: %v", err)
	}

	blocksDelta := targetHeight - latestHeight
	secondsDelta := float64(blocksDelta) * *avgSecs
	eta := latestTime.Add(time.Duration(secondsDelta * float64(time.Second)))
	sign := "+"
	if blocksDelta < 0 {
		sign = "-"
	}

	fmt.Printf("Current block : %s at %s\n", withCommasInt64(latestHeight), latestTime.UTC().Format(time.RFC3339))
	fmt.Printf("Target block  : %s\n", withCommasInt64(targetHeight))
	fmt.Printf("Avg block     : %.6f s\n", *avgSecs)
	fmt.Printf("\nΔblock        : %s%s\n", sign, withCommasInt64(absInt64(blocksDelta)))
	fmt.Printf("Estimated time: %s%s (%s s)\n", sign, elapsedDHMS(time.Duration(secondsDelta*float64(time.Second))), withCommasInt64(int64(math.Abs(secondsDelta))))
	fmt.Printf("\nEstimated target time:\n")
	fmt.Printf("  time        : %s (UTC)\n", eta.UTC().Format(time.RFC3339))
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

func parseTargetHeight(raw string) (int64, error) {
	height, err := strconv.ParseInt(strings.ReplaceAll(strings.TrimSpace(raw), ",", ""), 10, 64)
	if err != nil || height < 0 {
		return 0, fmt.Errorf("invalid target height %q (use a non-negative integer, e.g. 50185000)", raw)
	}
	return height, nil
}

func getLatest(ctx context.Context, c *http.Client, base string) (height int64, t time.Time, earliest int64, err error) {
	u := strings.TrimRight(base, "/") + "/status"
	var sr statusResp
	if err = getJSON(ctx, c, u, &sr); err != nil {
		return
	}
	height, err = strconv.ParseInt(sr.Result.SyncInfo.LatestBlockHeight, 10, 64)
	if err != nil {
		err = fmt.Errorf("parse latest height: %w", err)
		return
	}
	earliest, err = strconv.ParseInt(sr.Result.SyncInfo.EarliestBlockH, 10, 64)
	if err != nil {
		err = fmt.Errorf("parse earliest height: %w", err)
		return
	}
	t, err = time.Parse(time.RFC3339Nano, sr.Result.SyncInfo.LatestBlockTime)
	if err != nil {
		err = fmt.Errorf("parse latest time: %w", err)
	}
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
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("HTTP %d for %s", resp.StatusCode, url)
	}
	return json.NewDecoder(resp.Body).Decode(out)
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
