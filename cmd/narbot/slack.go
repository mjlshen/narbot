package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"narbot/pkg/endpoint"
	"net/http"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/slack-go/slack"
)

func helloWorld(s slack.SlashCommand, w http.ResponseWriter) {
	params := &slack.Msg{Text: s.Text}
	response := fmt.Sprintf("You asked for narbot :narwhal-happy: %v", params.Text)
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(response))
}

func EndpointSlackBlock(done <-chan interface{}, epStream <-chan endpoint.Endpoint) <-chan *slack.SectionBlock {
	slackBlocks := make(chan *slack.SectionBlock)
	go func() {
		defer close(slackBlocks)

		for e := range epStream {
			var block *slack.TextBlockObject
			if e.Healthy == true {
				block = slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("%s (%s) is healthy :white_check_mark:", e.Name, e.URL), false, false)
			} else {
				block = slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("%s (%s) is unhealthy :fire:", e.Name, e.URL), false, false)
			}
			epBlock := slack.NewSectionBlock(block, nil, nil)

			select {
			case <-done:
				return
			case slackBlocks <- epBlock:
			}
		}
	}()

	return slackBlocks
}

func monitorSerial(s slack.SlashCommand, c NarbotConfig, w http.ResponseWriter, start time.Time) {
	var blocks []slack.Block

	done := make(chan interface{})
	defer close(done)
	epStream := endpoint.CheckIsDown(done, endpoint.Generator(done, c.Endpoints...))

	for e := range epStream {
		var block *slack.TextBlockObject
		if e.Healthy == true {
			block = slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("%s (%s) is healthy :white_check_mark:", e.Name, e.URL), false, false)
		} else {
			block = slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("%s (%s) is unhealthy :fire:", e.Name, e.URL), false, false)
		}
		epBlock := slack.NewSectionBlock(block, nil, nil)
		blocks = append(blocks, epBlock)
	}

	divBlock := slack.NewDividerBlock()
	footerText := slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("It took me %s to check %d endpoints", time.Since(start), len(blocks)), false, false)
	footerBlock := slack.NewSectionBlock(footerText, nil, nil)
	blocks = append(blocks, divBlock)
	blocks = append(blocks, footerBlock)

	msg := slack.NewBlockMessage(blocks...)
	response, err := json.MarshalIndent(msg, "", "    ")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
	} else {
		http.Post(s.ResponseURL, "application/json", bytes.NewBuffer(response))
	}
}

func monitor(s slack.SlashCommand, c NarbotConfig, w http.ResponseWriter, start time.Time) {
	var blocks []slack.Block

	done := make(chan interface{})
	defer close(done)
	epStream := endpoint.Generator(done, c.Endpoints...)

	numCheckers := runtime.NumCPU()
	checkers := make([]<-chan endpoint.Endpoint, numCheckers)
	for i := 0; i < numCheckers; i++ {
		checkers[i] = endpoint.CheckIsDown(done, epStream)
	}

	fanIn := func(
		done <-chan interface{},
		channels ...<-chan endpoint.Endpoint,
	) <-chan endpoint.Endpoint {
		var wg sync.WaitGroup
		multiplexedStream := make(chan endpoint.Endpoint)

		multiplex := func(c <-chan endpoint.Endpoint) {
			defer wg.Done()
			for i := range c {
				select {
				case <-done:
					return
				case multiplexedStream <- i:
				}
			}
		}

		wg.Add(len(channels))
		for _, c := range channels {
			go multiplex(c)
		}

		go func() {
			wg.Wait()
			close(multiplexedStream)
		}()

		return multiplexedStream
	}

	for epBlock := range EndpointSlackBlock(done, fanIn(done, checkers...)) {
		blocks = append(blocks, epBlock)
	}

	divBlock := slack.NewDividerBlock()
	footerText := slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("It took me %s to check %d endpoints", time.Since(start), len(blocks)), false, false)
	footerBlock := slack.NewSectionBlock(footerText, nil, nil)
	blocks = append(blocks, divBlock)
	blocks = append(blocks, footerBlock)

	msg := slack.NewBlockMessage(blocks...)
	response, err := json.MarshalIndent(msg, "", "    ")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
	} else {
		http.Post(s.ResponseURL, "application/json", bytes.NewBuffer(response))
	}
}

func (narbotConfig *NarbotConfig) slashCommandHandler(w http.ResponseWriter, r *http.Request) {
	s, err := slashCommandVerifier(w, r, narbotConfig.SigningSecret)
	if err != nil {
		log.Println(err)
		return
	}

	// See which slash command the message contains
	switch s.Command {
	case "/narbot":
		params := strings.Split(s.Text, " ")
		switch params[0] {
		case "serial":
			w.WriteHeader(http.StatusOK)
			start := time.Now()
			monitorSerial(s, *narbotConfig, w, start)
		case "monitor":
			w.WriteHeader(http.StatusOK)
			start := time.Now()
			monitor(s, *narbotConfig, w, start)
		default:
			helloWorld(s, w)
		}
	// Unknown command
	default:
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func slashCommandVerifier(w http.ResponseWriter, r *http.Request, signingSecret string) (slashCommand slack.SlashCommand, err error) {
	var s slack.SlashCommand

	verifier, err := slack.NewSecretsVerifier(r.Header, signingSecret)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return s, fmt.Errorf("[ERROR] Failed to create secrets verifier: %v", err)
	}

	r.Body = ioutil.NopCloser(io.TeeReader(r.Body, &verifier))
	s, err = slack.SlashCommandParse(r)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return s, fmt.Errorf("[ERROR] Failed to parse slash command: %v", err)
	}

	if err = verifier.Ensure(); err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		return s, fmt.Errorf("[ERROR] Failed to validate against signing secret: %v", err)
	}

	return s, nil
}
