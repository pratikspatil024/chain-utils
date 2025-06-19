/*
How to run?
`go run heimdall_block_time_estimator.go`

What does it do?
TLDR: It estimates the time at which a particular block will be mined.
1. Get the current block height
2. Get the time at which this block was mined
3. Get the block time of a block which is 2000 blocks behind the current block
4. Calculate the average block time
5. Calculate the number of blocks left till the target block
6. Calculate the estimated time to reach the target block
*/
package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"time"
)

const (
	targetBlock = 8788500

	// to get the latest block
	latestSpanURL = "https://heimdall-api-amoy.polygon.technology/bor/latest-span"

	// to get the block creation time
	blockTimeURL = "https://tendermint-api-amoy.polygon.technology/block?height=%d"
)

func fetchHeight() (int, error) {
	resp, err := http.Get(latestSpanURL)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	var result struct {
		Height string `json:"height"`
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return 0, err
	}

	return strconv.Atoi(result.Height)
}

func fetchBlockTime(height int) (time.Time, error) {
	url := fmt.Sprintf(blockTimeURL, height)
	resp, err := http.Get(url)
	if err != nil {
		return time.Time{}, err
	}
	defer resp.Body.Close()

	var result struct {
		Result struct {
			BlockMeta struct {
				Header struct {
					Time string `json:"time"`
				} `json:"header"`
			} `json:"block_meta"`
		} `json:"result"`
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return time.Time{}, err
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return time.Time{}, err
	}

	return time.Parse(time.RFC3339Nano, result.Result.BlockMeta.Header.Time)
}

func main() {
	h1, err := fetchHeight()
	if err != nil {
		panic(err)
	}
	fmt.Println("Current height:", h1)

	t1, err := fetchBlockTime(h1)
	if err != nil {
		panic(err)
	}
	fmt.Println("Current block time:", t1)

	t2, err := fetchBlockTime(h1 - 2000)
	if err != nil {
		panic(err)
	}
	fmt.Println("Block time 2000 blocks ago:", t2)

	// Compute average block time
	avgBlockTime := t1.Sub(t2).Seconds() / 2000.0
	fmt.Printf("Average block time: %.2f seconds\n", avgBlockTime)

	blocksLeft := targetBlock - h1
	secondsLeft := avgBlockTime * float64(blocksLeft)

	estimatedTime := t1.Add(time.Duration(secondsLeft) * time.Second)
	fmt.Println("Estimated block mining time:", estimatedTime.Format(time.RFC3339Nano))
}
