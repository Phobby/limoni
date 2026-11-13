package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// The shared Lemon Drop board (apps/scoreboard, /drop/scores), same protocol
// as the terminal game (apps/lemondrop/remote.go): the client sends what the
// run did, the server recomputes the score. Only Classic runs are sent; the
// other modes have different rules and keep local boards.

const defaultBoard = "https://limoni-drab.vercel.app"

type remoteRun struct {
	Name   string     `json:"name"`
	Secs   int        `json:"secs"`
	Pieces int        `json:"pieces"`
	Drops  int        `json:"drops"`
	Clears [][2]int32 `json:"clears"`
}

type remoteAnswer struct {
	sent  bool
	board []entry
	rank  int
	err   error
}

type remote struct {
	url     string
	client  *http.Client
	answers chan remoteAnswer
}

func newRemote(url string) *remote {
	url = strings.TrimRight(strings.TrimSpace(url), "/")
	if url == "" || url == "off" {
		return nil
	}
	return &remote{url: url, client: &http.Client{Timeout: 70 * time.Second}, answers: make(chan remoteAnswer, 4)}
}

func (r *remote) fetch() {
	go func() {
		body, err := r.do("GET", nil)
		r.answer(remoteAnswer{}, body, err)
	}()
}

func (r *remote) submit(run remoteRun) {
	go func() {
		data, _ := json.Marshal(run)
		body, err := r.do("POST", data)
		r.answer(remoteAnswer{sent: true}, body, err)
	}()
}

func (r *remote) do(method string, body []byte) ([]byte, error) {
	req, err := http.NewRequest(method, r.url+"/drop/scores", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := r.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if err == nil && resp.StatusCode/100 != 2 {
		err = fmt.Errorf("scoreboard: %s", resp.Status)
	}
	return data, err
}

func (r *remote) answer(a remoteAnswer, body []byte, err error) {
	if err == nil {
		var got struct {
			Rank  int     `json:"rank"`
			Board []entry `json:"board"`
		}
		if err = json.Unmarshal(body, &got); err == nil {
			a.rank, a.board = got.Rank, got.Board
		}
	}
	a.err = err
	select {
	case r.answers <- a:
	default:
	}
}

// poll takes an answer if one has arrived; it never blocks.
func (r *remote) poll() (remoteAnswer, bool) {
	if r == nil {
		return remoteAnswer{}, false
	}
	select {
	case a := <-r.answers:
		return a, true
	default:
		return remoteAnswer{}, false
	}
}
