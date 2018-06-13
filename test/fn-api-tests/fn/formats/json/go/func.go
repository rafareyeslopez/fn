package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
)

type person struct {
	Name string `json:"name"`
}

type jason struct {
	Headers    http.Header `json:"headers"`
	Body       string      `json:"body,omitempty"`
	StatusCode int         `json:"status,omitempty"`
}

func main() {

	stdin := json.NewDecoder(os.Stdin)
	stdout := json.NewEncoder(os.Stdout)
	stderr := json.NewEncoder(os.Stderr)
	for {
		in := &jason{}

		err := stdin.Decode(in)
		if err != nil {
			log.Fatalf("Unable to decode incoming data: %s", err.Error())
			fmt.Fprintf(os.Stderr, err.Error())
		}
		p := person{}
		stderr.Encode(in.Body)
		if len(in.Body) != 0 {
			if err := json.NewDecoder(bytes.NewReader([]byte(in.Body))).Decode(&p); err != nil {
				log.Fatalf("Unable to decode Person object data: %s", err.Error())
				fmt.Fprintf(os.Stderr, err.Error())
			}
		}
		if p.Name == "" {
			p.Name = "World"
		}

		mapResult := map[string]string{"message": fmt.Sprintf("Hello %s", p.Name)}
		b, err := json.Marshal(mapResult)
		if err != nil {
			log.Fatalf("Unable to marshal JSON response body: %s", err.Error())
			fmt.Fprintf(os.Stderr, err.Error())
		}
		h := http.Header{}
		h.Set("Content-Type", "application/json")
		h.Set("Content-Length", strconv.Itoa(len(b)))
		out := &jason{
			StatusCode: http.StatusOK,
			Body:       string(b),
			Headers:    h,
		}
		stderr.Encode(out)
		if err := stdout.Encode(out); err != nil {
			log.Fatalf("Unable to encode JSON response: %s", err.Error())
			fmt.Fprintf(os.Stderr, err.Error())
		}
	}
}
