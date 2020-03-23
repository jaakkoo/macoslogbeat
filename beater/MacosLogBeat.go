package beater

import (
	"bufio"
	"fmt"
	"log"
	"os/exec"
	"time"

	"github.com/bcicen/jstream"
	"github.com/elastic/beats/libbeat/beat"
	"github.com/elastic/beats/libbeat/common"
	"github.com/elastic/beats/libbeat/logp"
	"github.com/jaakkoo/macoslogbeat/config"
)

// macoslogbeat configuration.
type macoslogbeat struct {
	done   chan struct{}
	config config.Config
	client beat.Client
}

// New creates an instance of macoslogbeat.
func New(b *beat.Beat, cfg *common.Config) (beat.Beater, error) {
	c := config.DefaultConfig
	if err := cfg.Unpack(&c); err != nil {
		return nil, fmt.Errorf("Error reading config file: %v", err)
	}

	bt := &macoslogbeat{
		done:   make(chan struct{}),
		config: c,
	}
	return bt, nil
}

// Run starts macoslogbeat.
func (bt *macoslogbeat) Run(b *beat.Beat) error {
	logp.Info("macoslogbeat is running! Hit CTRL-C to stop it.")
	app := "/usr/bin/log"
	cmd := exec.Command(app, "stream", "--style=json", "--predicate", "(subsystem != \"com.apple.GPUWrangler\") and (subsystem != \"com.apple.bluetooth\")")

	var err error

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Fatal(err)
	}

	bt.client, err = b.Publisher.Connect()
	if err != nil {
		return err
	}

	ticker := time.NewTicker(bt.config.Period)

	cmd.Start()
	reader := bufio.NewReader(stdout)
	// Read the first line from buffer because it's not json
	reader.ReadBytes('\n')
	dec := jstream.NewDecoder(reader, 1)

	for {
		select {
		case <-bt.done:
			return nil
		case <-ticker.C:
		}

		for mv := range dec.Stream() {
			event := beat.Event{
				Timestamp: time.Now(),
				Fields:    common.MapStr(mv.Value.(map[string]interface{})),
			}
			bt.client.Publish(event)
		}
	}
}

// Stop stops macoslogbeat.
func (bt *macoslogbeat) Stop() {
	bt.client.Close()
	close(bt.done)
}
