// go run bor_average_blocktime_calculator.go
// go run bor_average_blocktime_calculator.go -rpc="https://polygon-rpc.com"

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
	"strings"
	"time"
)

const (
	defaultRPC   = "https://polygon-rpc.com"
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
	Number    string `json:"number"`
	Timestamp string `json:"timestamp"`
}

func main() {
	rpcURL := flag.String("rpc", defaultRPC, "Polygon (Bor) JSON-RPC endpoint")
	flag.Parse()

	client := &http.Client{Timeout: httpTimeout}
	ctx := context.Background()

	// 1) latest block n
	n, err := getLatestBlockNumber(ctx, client, *rpcURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: get latest block number: %v\n", err)
		os.Exit(1)
	}

	// 2) targets {n, n-40000, n-280000, n-560000, n-1120000}
	targets := []target{
		{kind: "relative", delta: 0},
		{kind: "relative", delta: -40000},
		{kind: "relative", delta: -280000},
		{kind: "relative", delta: -560000},
		{kind: "relative", delta: -1120000},
	}

	// Resolve valid heights (skip negatives/future)
	var heights []uint64
	for _, t := range targets {
		if h, ok := t.resolve(n); ok {
			heights = append(heights, h)
		}
	}

	// Fetch timestamps
	type info struct {
		height    uint64
		timestamp uint64
	}
	infos := make(map[uint64]info)
	for _, h := range heights {
		ts, err := getBlockTimestamp(ctx, client, *rpcURL, h)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to fetch block %d: %v\n", h, err)
			continue
		}
		infos[h] = info{height: h, timestamp: ts}
	}

	// Ensure n present
	nTS, ok := func() (uint64, bool) {
		if x, ok := infos[n]; ok {
			return x.timestamp, true
		}
		ts, err := getBlockTimestamp(ctx, client, *rpcURL, n)
		if err != nil {
			return 0, false
		}
		infos[n] = info{height: n, timestamp: ts}
		return ts, true
	}()
	if !ok {
		fmt.Fprintf(os.Stderr, "error: failed to fetch latest block %d timestamp\n", n)
		os.Exit(1)
	}

	// 5) Pretty header for current block
	fmt.Printf("Current block: %s — %s (UTC)\n",
		withCommas(n),
		isoTime(infos[n].timestamp),
	)

	// 6) Pretty per-reference output
	for _, t := range targets {
		h, ok := t.resolve(n)
		if !ok || h == n {
			continue
		}
		src, ok := infos[h]
		if !ok {
			continue
		}

		blockDiff := int64(n) - int64(h)
		secDiff := int64(nTS) - int64(src.timestamp)
		avg := math.NaN()
		if blockDiff != 0 {
			avg = float64(secDiff) / float64(blockDiff)
		}

		// First line: Δ<blocks> from <h> (<iso>) → <n>
		deltaLabel := fmt.Sprintf("Δ%d", blockDiff) // no commas to match the inspiration
		fmt.Printf("\n%-10s from height %s (%s)  \u2192  %s\n",
			deltaLabel,
			withCommas(h),
			isoTime(src.timestamp),
			withCommas(n),
		)

		// Second line: elapsed (as 0d Xh Ym Zs, always showing units)
		fmt.Printf("  elapsed    : %s\n", elapsedDHMS(secDiff))

		// Third line: avg block time (seconds + milliseconds)
		fmt.Printf("  avg block  : %.6f s/block  (%.3f ms)\n",
			avg,
			avg*1000.0,
		)
	}
}

type target struct {
	kind  string // "relative" or "absolute"
	delta int64  // for relative
	value uint64 // for absolute
}

func (t target) resolve(n uint64) (uint64, bool) {
	switch t.kind {
	case "relative":
		if t.delta >= 0 {
			return n + uint64(t.delta), true
		}
		d := uint64(-t.delta)
		if d > n {
			return 0, false
		}
		return n - d, true
	case "absolute":
		if t.value > n {
			return 0, false
		}
		return t.value, true
	default:
		return 0, false
	}
}

func getLatestBlockNumber(ctx context.Context, client *http.Client, rpcURL string) (uint64, error) {
	var hex string
	if err := rpcCall(ctx, client, rpcURL, "eth_blockNumber", []interface{}{}, &hex); err != nil {
		return 0, err
	}
	return hexToUint64(hex)
}

func getBlockTimestamp(ctx context.Context, client *http.Client, rpcURL string, height uint64) (uint64, error) {
	hexHeight := fmt.Sprintf("0x%x", height)
	params := []interface{}{hexHeight, false}
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
		reqBody := rpcRequest{
			JSONRPC: jsonrpcVer,
			Method:  method,
			Params:  params,
			ID:      1,
		}
		b, _ := json.Marshal(reqBody)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, rpcURL, bytes.NewReader(b))
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

		var decoded rpcResponse[T]
		dec := json.NewDecoder(resp.Body)
		err = dec.Decode(&decoded)
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
	if strings.HasPrefix(h, "0x") || strings.HasPrefix(h, "0X") {
		h = h[2:]
	}
	if h == "" {
		return 0, fmt.Errorf("empty hex string")
	}
	bi := new(big.Int)
	if _, ok := bi.SetString(h, 16); !ok {
		return 0, fmt.Errorf("invalid hex %q", h)
	}
	if bi.Sign() < 0 || !bi.IsUint64() {
		return 0, fmt.Errorf("hex %q out of uint64 range", h)
	}
	return bi.Uint64(), nil
}

func withCommas(u uint64) string {
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

func isoTime(unixSec uint64) string {
	return time.Unix(int64(unixSec), 0).UTC().Format(time.RFC3339)
}

func elapsedDHMS(totalSec int64) string {
	if totalSec < 0 {
		totalSec = -totalSec
	}
	d := totalSec / 86400
	r := totalSec % 86400
	h := r / 3600
	r %= 3600
	m := r / 60
	s := r % 60
	return fmt.Sprintf("%dd %dh %dm %ds", d, h, m, s)
}
