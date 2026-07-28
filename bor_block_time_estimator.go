// go run bor_block_time_estimator.go -rpc="$BOR_MAINNET_RPC" -target=80000000 -avg=2.156
// go run bor_block_time_estimator.go help

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"math"
	"math/big"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	jsonrpcVer   = "2.0"
	httpTimeout  = 20 * time.Second
	maxRetries   = 3
	retryBackoff = 600 * time.Millisecond
)

type rpcRequest struct {
	JSONRPC string        `json:"jsonrpc"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
	ID      int           `json:"id"`
}

type rpcResponse[T any] struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int    `json:"id"`
	Result  T      `json:"result"`
	Error   *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

type block struct {
	Timestamp string `json:"timestamp"`
}

func main() {
	fs := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	rpcURL := fs.String("rpc", "", "Bor JSON-RPC endpoint")
	targetStr := fs.String("target", "", "Target Bor block height")
	avgSecs := fs.Float64("avg", 0, "Average block time in seconds")
	timeout := fs.Duration("timeout", httpTimeout, "HTTP request timeout")
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage:\n  go run bor_block_time_estimator.go -rpc=<bor-rpc-url> -target=<block-height> -avg=<seconds> [options]\n  go run bor_block_time_estimator.go help\n\n")
		fmt.Fprintln(fs.Output(), "Predicts when Bor will reach a target block height using the current chain head and a supplied average block time.")
		fmt.Fprintln(fs.Output(), "\nRequired:")
		fmt.Fprintln(fs.Output(), "  -rpc string")
		fmt.Fprintln(fs.Output(), "        Bor JSON-RPC endpoint for the network being scheduled")
		fmt.Fprintln(fs.Output(), "  -target string")
		fmt.Fprintln(fs.Output(), "        Target Bor block height, for example 80000000")
		fmt.Fprintln(fs.Output(), "  -avg float")
		fmt.Fprintln(fs.Output(), "        Average block time in seconds, usually copied from bor_average_blocktime_calculator.go")
		fmt.Fprintln(fs.Output(), "\nOptions:")
		fs.PrintDefaults()
		fmt.Fprintln(fs.Output(), "\nExamples:")
		fmt.Fprintln(fs.Output(), "  go run bor_block_time_estimator.go -rpc=$BOR_MAINNET_RPC -target=80000000 -avg=2.156")
		fmt.Fprintln(fs.Output(), "  go run bor_block_time_estimator.go -rpc=https://rpc-amoy.polygon.technology -target=30000000 -avg=2.1")
	}
	if len(os.Args) > 1 && isHelpCommand(os.Args[1]) {
		fs.Usage()
		return
	}
	if err := fs.Parse(os.Args[1:]); err != nil {
		failf("%v", err)
	}
	missing := missingRequiredFlags(fs, *rpcURL, *targetStr)
	if len(missing) > 0 {
		failf("missing required options: %s\n\nRequired inputs:\n  -rpc    Bor JSON-RPC endpoint for the network being scheduled.\n  -target Target Bor block height, for example 80000000.\n  -avg    Positive average Bor block time in seconds, usually copied from `bor_average_blocktime_calculator.go`.\n\nExample:\n  go run bor_block_time_estimator.go -rpc=$BOR_MAINNET_RPC -target=80000000 -avg=2.156\n\nRun `go run bor_block_time_estimator.go help` for all options.", strings.Join(missing, ", "))
	}
	if *avgSecs <= 0 {
		failf("invalid -avg: %.6f\n\nProvide a positive average Bor block time in seconds, usually copied from `bor_average_blocktime_calculator.go`.\nExample:\n  -avg=2.156\n\nRun `go run bor_block_time_estimator.go help` for all options.", *avgSecs)
	}
	targetHeight, err := parseTargetHeight(*targetStr)
	if err != nil {
		failf("parse target height: %v", err)
	}

	ctx := context.Background()
	client := &http.Client{Timeout: *timeout}
	latestHeight, err := getLatestBlockNumber(ctx, client, *rpcURL)
	if err != nil {
		failf("get latest block number: %v", err)
	}
	latestTimestamp, err := getBlockTimestamp(ctx, client, *rpcURL, latestHeight)
	if err != nil {
		failf("get timestamp for current block %d: %v", latestHeight, err)
	}
	latestTime := time.Unix(int64(latestTimestamp), 0).UTC()

	blocksDelta := targetHeight - int64(latestHeight)
	secondsDelta := float64(blocksDelta) * *avgSecs
	eta := latestTime.Add(time.Duration(secondsDelta * float64(time.Second)))
	sign := "+"
	if blocksDelta < 0 {
		sign = "-"
	}

	fmt.Printf("Current block : %s at %s\n", withCommasUint64(latestHeight), latestTime.Format(time.RFC3339))
	fmt.Printf("Target block  : %s\n", withCommasInt64(targetHeight))
	fmt.Printf("Avg block     : %.6f s\n", *avgSecs)
	fmt.Printf("\nΔblock        : %s%s\n", sign, withCommasInt64(absInt64(blocksDelta)))
	fmt.Printf("Estimated time: %s%s (%s s)\n", sign, elapsedDHMS(time.Duration(secondsDelta*float64(time.Second))), withCommasInt64(int64(math.Abs(secondsDelta))))
	fmt.Printf("\nEstimated target time:\n")
	fmt.Printf("  time        : %s (UTC)\n", eta.Format(time.RFC3339))
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
		return 0, fmt.Errorf("invalid target height %q (use a non-negative integer, e.g. 80000000)", raw)
	}
	return height, nil
}

func getLatestBlockNumber(ctx context.Context, client *http.Client, rpcURL string) (uint64, error) {
	var hex string
	if err := rpcCall(ctx, client, rpcURL, "eth_blockNumber", []interface{}{}, &hex); err != nil {
		return 0, err
	}
	return hexToUint64(hex)
}

func getBlockTimestamp(ctx context.Context, client *http.Client, rpcURL string, height uint64) (uint64, error) {
	params := []interface{}{fmt.Sprintf("0x%x", height), false}
	var respBlock *block
	if err := rpcCall(ctx, client, rpcURL, "eth_getBlockByNumber", params, &respBlock); err != nil {
		return 0, err
	}
	if respBlock == nil || respBlock.Timestamp == "" {
		return 0, fmt.Errorf("empty block/timestamp for height %d", height)
	}
	return hexToUint64(respBlock.Timestamp)
}

func rpcCall[T any](ctx context.Context, client *http.Client, rpcURL, method string, params []interface{}, out *T) error {
	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		reqBody := rpcRequest{JSONRPC: jsonrpcVer, Method: method, Params: params, ID: 1}
		body, _ := json.Marshal(reqBody)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, rpcURL, bytes.NewReader(body))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			time.Sleep(retryBackoff * time.Duration(attempt+1))
			continue
		}
		if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
			lastErr = fmt.Errorf("HTTP %d", resp.StatusCode)
			resp.Body.Close()
			time.Sleep(retryBackoff * time.Duration(attempt+1))
			continue
		}
		var decoded rpcResponse[T]
		err = json.NewDecoder(resp.Body).Decode(&decoded)
		resp.Body.Close()
		if err != nil {
			lastErr = err
			time.Sleep(retryBackoff * time.Duration(attempt+1))
			continue
		}
		if decoded.Error != nil {
			lastErr = errors.New(decoded.Error.Message)
			time.Sleep(retryBackoff * time.Duration(attempt+1))
			continue
		}
		*out = decoded.Result
		return nil
	}
	return fmt.Errorf("rpc %s failed after %d attempts: %v", method, maxRetries, lastErr)
}

func hexToUint64(h string) (uint64, error) {
	h = strings.TrimPrefix(strings.TrimPrefix(h, "0x"), "0X")
	if h == "" {
		return 0, errors.New("empty hex string")
	}
	bi := new(big.Int)
	if _, ok := bi.SetString(h, 16); !ok || bi.Sign() < 0 || !bi.IsUint64() {
		return 0, fmt.Errorf("invalid hex %q", h)
	}
	return bi.Uint64(), nil
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

func withCommasUint64(u uint64) string {
	s := fmt.Sprintf("%d", u)
	if len(s) <= 3 {
		return s
	}
	var b strings.Builder
	pre := len(s) % 3
	if pre == 0 {
		pre = 3
	}
	b.WriteString(s[:pre])
	for i := pre; i < len(s); i += 3 {
		b.WriteByte(',')
		b.WriteString(s[i : i+3])
	}
	return b.String()
}

func withCommasInt64(v int64) string {
	if v < 0 {
		return "-" + withCommasUint64(uint64(-v))
	}
	return withCommasUint64(uint64(v))
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
